package app

import (
	"context"
	"errors"
	"log"
	"os"
	"slices"
	"time"

	"example.com/app/internal/analytics"
	"example.com/app/internal/risk"
	"example.com/app/internal/source"
)

const (
	// defaultFullEvery — как часто полный пересчёт сверяет накопленное частичными (RECALC_FULL_EVERY).
	defaultFullEvery = 60 * time.Second
	// partialMaxShare — выше этой доли клиентов в одной пачке событий частичный пересчёт не выигрывает:
	// считаем полный (читает таблицы один раз и не копирует портфель на каждого).
	partialMaxShare = 0.05
)

// fullEveryFromEnv читает RECALC_FULL_EVERY (формат time.ParseDuration). 0 — всегда полный пересчёт,
// то есть частичный выключен. Пусто или неверно — 60 с, неверное значение попадает в лог.
func fullEveryFromEnv() time.Duration {
	v := os.Getenv("RECALC_FULL_EVERY")
	if v == "" {
		return defaultFullEvery
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		log.Printf("app: RECALC_FULL_EVERY=%q не длительность вида 60s, беру %s", v, defaultFullEvery)
		return defaultFullEvery
	}
	return d
}

// touchedAccounts достаёт из source.changed клиентов пачки и версию данных. Остальные события
// (model.changed) и снимок целиком клиентов не называют: пересчёт полный.
func touchedAccounts(topic string, payload any) ([]int64, int64) {
	if topic != source.TopicChanged {
		return nil, 0
	}
	ch, ok := payload.(source.Change)
	if !ok {
		return nil, 0
	}
	return ch.Accounts, ch.Version
}

// partialConfig — частичный пересчёт для воркера: клиенты события, сводка разностью, портфель с заменой строк.
func (r *Recalc) partialConfig(fullEvery time.Duration) analytics.Partial[risk.Result] {
	return analytics.Partial[risk.Result]{
		Touched:   touchedAccounts,
		Compute:   r.computePartial,
		FullEvery: fullEvery,
		MaxShare:  partialMaxShare,
		Total:     func(res risk.Result) int { return len(res.Customers) },
	}
}

// WithFullEvery задаёт период полного пересчёта вместо RECALC_FULL_EVERY; 0 выключает частичный.
// Вызывать до Start.
func (r *Recalc) WithFullEvery(d time.Duration) *Recalc {
	r.Engine.WithPartial(r.partialConfig(d))
	return r
}

// computePartial пересчитывает клиентов ids поверх прежнего результата: читает из базы только их
// строки, считает только их, правит сводку разностью и меняет их строки в портфеле. Любое сомнение
// (настройки сменились, asOf другой, клиента нет в прежнем портфеле) — analytics.ErrNeedFull.
func (r *Recalc) computePartial(ctx context.Context, prev *analytics.Calculation[risk.Result], ids []int64, dataVersion, version int64) (analytics.Computed[risk.Result], error) {
	st, settingsVersion, err := currentSettings(ctx, r.Settings)
	if err != nil {
		return analytics.Computed[risk.Result]{}, err
	}
	if settingsVersion != prev.SettingsVersion {
		return analytics.Computed[risk.Result]{}, analytics.ErrNeedFull
	}
	prevPF := r.Portfolio()
	if prevPF == nil {
		return analytics.Computed[risk.Result]{}, analytics.ErrNeedFull
	}
	var stages analytics.Stages
	t0 := r.clk.Now()
	rows, loaded, err := r.reader.LoadAccounts(ctx, ids)
	if err != nil {
		return analytics.Computed[risk.Result]{}, err
	}
	stages.Load = r.clk.Now().Sub(t0)
	t1 := r.clk.Now()
	rs := ToRiskSnapshot(rows)
	stages.Convert = r.clk.Now().Sub(t1)

	t2 := r.clk.Now()
	res, err := risk.Rescore(ctx, prev.Result, rs, ids, st, risk.Options{OnBatch: r.metrics.ObserveBatch})
	if errors.Is(err, risk.ErrNeedFull) {
		return analytics.Computed[risk.Result]{}, analytics.ErrNeedFull
	}
	if err != nil {
		return analytics.Computed[risk.Result]{}, err
	}
	stages.Score = r.clk.Now().Sub(t2)

	pf := analytics.PatchPortfolio(prevPF, version, changedCustomers(res, rs))
	if pf == nil {
		return analytics.Computed[risk.Result]{}, analytics.ErrNeedFull
	}
	if dataVersion == 0 {
		dataVersion = loaded
	}
	return analytics.Computed[risk.Result]{
		Result:          res,
		DataVersion:     dataVersion,
		SettingsVersion: settingsVersion,
		Stages:          stages,
		Customers:       len(rs.Accounts),
		Extra:           pf,
	}, nil
}

// changedCustomers собирает для портфеля клиентов из строк rs по их новому результату.
func changedCustomers(res risk.Result, rs risk.Snapshot) []analytics.Customer {
	svcs := map[int64][]risk.Service{}
	for _, sv := range rs.Services {
		svcs[sv.OwnerID] = append(svcs[sv.OwnerID], sv)
	}
	out := make([]analytics.Customer, 0, len(rs.Accounts))
	for _, a := range rs.Accounts {
		i, ok := slices.BinarySearchFunc(res.Customers, a.ID, func(c risk.Customer, id int64) int {
			switch {
			case c.ID < id:
				return -1
			case c.ID > id:
				return 1
			}
			return 0
		})
		if ok {
			out = append(out, buildCustomer(res.Customers[i], a, svcs[a.ID]))
		}
	}
	return out
}

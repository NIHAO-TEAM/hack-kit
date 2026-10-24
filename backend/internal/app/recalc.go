package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"example.com/app/internal/analytics"
	"example.com/app/internal/model"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/retention"
	"example.com/app/internal/risk"
	"example.com/app/internal/source"
)

// Recalc — воркер пересчёта и то, что он читает. Модули друг о друге не знают, поэтому
// склейку «снимок и настройки → ядро модели» делает сборка сервера.
type Recalc struct {
	Engine   *analytics.Engine[risk.Result]
	Settings *model.Store
	snaps    *snapshots
	cache    cacheHolder
	observer *retention.Service
	reads    *atomic.Int64 // сколько раз пересчёт читал таблицы снимка целиком
	metrics  *analytics.RecalcMetrics
}

// NewRecalc собирает воркер: настройки из model_settings, снимок из таблиц организаторов.
func NewRecalc(pool *pgxpool.Pool, bus *events.Bus, clk clock.Clock) *Recalc {
	snaps := &snapshots{}
	reads := &atomic.Int64{}
	metrics := analytics.NewRecalcMetrics(risk.Workers)
	store := model.NewStore(pool, bus, ValidateSettings)
	reader := source.NewReader(pool)
	compute := func(ctx context.Context) (analytics.Computed[risk.Result], error) {
		st, settingsVersion, err := currentSettings(ctx, store)
		if err != nil {
			return analytics.Computed[risk.Result]{}, err
		}
		var stages analytics.Stages
		t0 := clk.Now()
		// Версия снимка та же — таблицы не перечитываем: любое изменение данных заводит новую версию.
		dataVersion, err := reader.Version(ctx)
		if err != nil {
			return analytics.Computed[risk.Result]{}, err
		}
		rs, ok := snaps.get(dataVersion)
		if !ok {
			reads.Add(1)
			snap, loaded, err := reader.Load(ctx)
			if err != nil {
				return analytics.Computed[risk.Result]{}, err
			}
			dataVersion = loaded
			stages.Load = clk.Now().Sub(t0)
			t1 := clk.Now()
			rs = ToRiskSnapshot(snap)
			stages.Convert = clk.Now().Sub(t1)
			snaps.put(dataVersion, rs)
		} else {
			stages.Load = clk.Now().Sub(t0)
		}
		t2 := clk.Now()
		res := risk.Score(rs, st, rs.AsOf)
		stages.Score = clk.Now().Sub(t2)
		return analytics.Computed[risk.Result]{
			Result:          res,
			DataVersion:     dataVersion,
			SettingsVersion: settingsVersion,
			Stages:          stages,
		}, nil
	}
	r := &Recalc{
		Engine:   analytics.NewEngine(compute, bus, clk, source.TopicChanged, model.TopicChanged),
		Settings: store,
		snaps:    snaps,
		reads:    reads,
		metrics:  metrics,
	}
	metrics.Cancelled = r.Engine.Cancelled
	r.observer = retention.New(pool, bus, clk, r)
	return r
}

// Collectors — метрики пересчёта для /metrics: воркеры, отменённые прогоны, длительность пачек.
func (r *Recalc) Collectors() []prometheus.Collector { return []prometheus.Collector{r.metrics} }

// TableReads — сколько раз пересчёт читал таблицы снимка целиком; смена настроек их не читает.
func (r *Recalc) TableReads() int64 { return r.reads.Load() }

// Start запускает воркер: первый пересчёт идёт сразу, дальше по событиям.
// Вместе с ним работает наблюдатель за открытыми задачами удержания: он читает результат каждого пересчёта.
// Канал закрывается, когда остановились оба.
func (r *Recalc) Start(ctx context.Context) <-chan struct{} {
	observed := r.observer.StartObserver(ctx) // подписка до первого расчёта, иначе его событие потеряется
	engine := r.Engine.Start(ctx)
	stopped := make(chan struct{})
	go func() {
		<-engine
		<-observed
		close(stopped)
	}()
	return stopped
}

// ValidateSettings проверяет тело настроек модели.
func ValidateSettings(body []byte) error {
	var st risk.Settings
	if err := json.Unmarshal(body, &st); err != nil {
		return fmt.Errorf("разбор настроек: %w", err)
	}
	return st.Validate()
}

// currentSettings читает сохранённые настройки; пока их не сохраняли — умолчания, версия 0.
func currentSettings(ctx context.Context, store *model.Store) (risk.Settings, int64, error) {
	body, version, err := store.Current(ctx)
	if err != nil {
		return risk.Settings{}, 0, err
	}
	if body == nil {
		return risk.Defaults(), version, nil
	}
	var st risk.Settings
	if err := json.Unmarshal(body, &st); err != nil {
		return risk.Settings{}, 0, fmt.Errorf("настройки версии %d: %w", version, err)
	}
	return st, version, nil
}

// ToRiskSnapshot переводит снимок источника во входные данные ядра модели.
func ToRiskSnapshot(s source.Snapshot) risk.Snapshot {
	out := risk.Snapshot{
		Accounts:     make([]risk.Account, len(s.Accounts)),
		Services:     make([]risk.Service, len(s.Services)),
		Transactions: make([]risk.Transaction, len(s.Transactions)),
	}
	if s.AsOf != nil {
		out.AsOf = *s.AsOf
	}
	for i, a := range s.Accounts {
		out.Accounts[i] = risk.Account(a)
	}
	for i, v := range s.Services {
		out.Services[i] = risk.Service(v)
	}
	for i, t := range s.Transactions {
		out.Transactions[i] = risk.Transaction(t)
	}
	return out
}

// Overview переводит последний результат пересчёта в сводку для analytics; nil, пока расчёта нет.
func (r *Recalc) Overview() *analytics.Overview {
	if r == nil {
		return nil
	}
	c := r.Engine.Latest()
	if c == nil {
		return nil
	}
	s := c.Result.Summary
	return &analytics.Overview{
		Version:           c.Version,
		AsOf:              c.Result.AsOf,
		CalculatedAt:      c.CalculatedAt,
		Duration:          c.Duration,
		Stages:            c.Stages,
		SettingsVersion:   c.SettingsVersion,
		Levels:            s.Levels,
		ExpectedLossCents: s.ExpectedLossCents,
		AtRiskCents:       s.ExposureCents,
		LostCents:         s.LostCents,
		QualityCounts:     s.QualityCounts,
	}
}

// QualityChecks — результаты проверок качества данных последнего пересчёта по снимку этой версии;
// nil, пока пересчёт по ней не готов (тогда экран показывает «проверки считаются»).
func (r *Recalc) QualityChecks(version int64) []gen.QualityCheck {
	if r == nil {
		return nil
	}
	c := r.Engine.Latest()
	if c == nil || c.DataVersion != version {
		return nil
	}
	return qualityChecks(c.Result.Checks)
}

func qualityChecks(in []risk.CheckResult) []gen.QualityCheck {
	out := make([]gen.QualityCheck, 0, len(in))
	for _, c := range in {
		msg := c.Message
		q := gen.QualityCheck{Id: c.ID, Ok: c.OK, Message: &msg}
		if len(c.Items) > 0 {
			items := make([]gen.QualityCheckItem, len(c.Items))
			for i, it := range c.Items {
				items[i] = gen.QualityCheckItem{ServiceId: it.ServiceID, AccountId: it.AccountID}
			}
			q.Items = &items
		}
		out = append(out, q)
	}
	return out
}

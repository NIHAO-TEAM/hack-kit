package risk

import (
	"cmp"
	"context"
	"math"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Состояния клиента.
const (
	StateScored   = "scored"
	StateUnscored = "unscored"
	StateLost     = "lost"
)

// Уровни риска.
const (
	LevelHigh   = "high"
	LevelMedium = "medium"
	LevelLow    = "low"
)

// FeatureHit — вклад признака в вероятность непродления услуги.
// Contribution = вес · f: слагаемое под произведением 1 − Π(1 − w·f).
type FeatureHit struct {
	ID           string
	F            float64
	Weight       float64
	Contribution float64
	Reason       string
}

// ServiceResult — результат по живой услуге. Features — по убыванию вклада, при равенстве по ID.
type ServiceResult struct {
	ID        int64
	P         float64
	RCents    int64
	LossCents int64
	Features  []FeatureHit
	Quality   []string
	// QualityIDs — идентификаторы сработавших проверок, в порядке Quality.
	QualityIDs []string
}

// ServiceQuality — флаги качества неживой услуги (заблокированной, удалённой): в скор не входит.
type ServiceQuality struct {
	ServiceID int64
	Flags     []string
	IDs       []string // идентификаторы проверок, в порядке Flags
}

// ActionResult — действие с ценой: эффект = ожидаемая потеря после − до (отрицательный — выгода).
type ActionResult struct {
	ID             string
	Title          string
	LossDeltaCents int64
	ScoreBefore    *float64
	ScoreAfter     *float64
	LevelAfter     *string
}

// Customer — результат по клиенту. ExposureCents — Σ R по живым услугам клиента.
type Customer struct {
	ID                int64
	State             string
	Score             *float64
	Level             *string
	ExpectedLossCents int64
	ExposureCents     int64
	LostCents         int64
	Services          []ServiceResult
	Quality           []ServiceQuality
	Actions           []ActionResult
}

// Summary — сводка. ExposureCents («под угрозой») — только клиенты high и medium.
type Summary struct {
	Levels            map[string]int
	ExpectedLossCents int64
	ExposureCents     int64
	LostCents         int64
	QualityCounts     map[string]int
	// LossByFeature — ожидаемая потеря по причинам; сумма равна ExpectedLossCents.
	LossByFeature []FeatureLoss
}

// CheckItem — услуга, на которой сработала проверка.
type CheckItem struct {
	ServiceID int64
	AccountID int64
}

// maxCheckItems — сколько услуг проверки отдаём списком; Count всегда полный.
const maxCheckItems = 10

// CheckResult — итог одной проверки качества данных по снимку.
type CheckResult struct {
	ID      string
	OK      bool
	Message string
	Count   int
	Items   []CheckItem
}

// Result — результат расчёта по снимку.
type Result struct {
	AsOf      time.Time
	Customers []Customer
	Summary   Summary
	Checks    []CheckResult // по одной записи на каждую зарегистрированную проверку, по ID
}

// Score считает модель по правилам docs/model-rules.md. Чистая функция: часов не читает, ввода-вывода нет.
func Score(s Snapshot, st Settings, asOf time.Time) Result {
	return scoreWith(s, st, asOf, features.all(), checks.all(), actions.all())
}

// Options — необязательные настройки конвейера: размер пачки и наблюдатель за пачками (метрики).
// Нулевое значение — пачка по умолчанию и без наблюдателя; на результат расчёта не влияет.
type Options struct {
	BatchSize int
	OnBatch   func(clients int, took time.Duration)
}

// DefaultBatchSize — сколько клиентов воркер берёт за раз (подобрано бенчмарком, docs/load).
const DefaultBatchSize = 256

// Workers — сколько воркеров считает n клиентов: по числу ядер, но не больше n.
func Workers(n int) int {
	w := runtime.GOMAXPROCS(0)
	if w > n {
		w = n
	}
	if w < 1 {
		w = 1
	}
	return w
}

// ScoreContext — Score, который можно отменить: воркеры проверяют ctx между пачками.
// Отменённый расчёт возвращает ошибку контекста и пустой результат — частичного не бывает.
func ScoreContext(ctx context.Context, s Snapshot, st Settings, asOf time.Time, opt Options) (Result, error) {
	return scoreCtx(ctx, opt, s, st, asOf, features.all(), checks.all(), actions.all())
}

func scoreWith(s Snapshot, st Settings, asOf time.Time, fs []Feature, cs []Check, as []Action) Result {
	res, _ := scoreCtx(context.Background(), Options{}, s, st, asOf, fs, cs, as)
	return res
}

func scoreCtx(ctx context.Context, opt Options, s Snapshot, st Settings, asOf time.Time, fs []Feature, cs []Check, as []Action) (Result, error) {
	svcByOwner := groupBy(s.Services, func(sv Service) int64 { return sv.OwnerID })
	txByService := groupBy(s.Transactions, func(t Transaction) int64 { return t.ServiceID })

	accounts := append([]Account(nil), s.Accounts...)
	slices.SortFunc(accounts, func(a, b Account) int { return cmp.Compare(a.ID, b.ID) })

	res := Result{AsOf: asOf, Summary: Summary{
		Levels:        map[string]int{LevelHigh: 0, LevelMedium: 0, LevelLow: 0, StateLost: 0, StateUnscored: 0},
		QualityCounts: map[string]int{},
	}}
	hits := map[string]*CheckResult{}
	for _, c := range cs {
		hits[c.ID()] = &CheckResult{ID: c.ID(), OK: true}
	}
	// Клиенты не зависят друг от друга: считаем их параллельно в слайс по индексу, а сводку и
	// проверки собираем потом по порядку индекса — результат тот же, что при последовательном счёте.
	customers := make([]Customer, len(accounts))
	err := parallelFor(ctx, len(accounts), opt, func(w *scratch, i int) {
		a := accounts[i]
		c, funds := scoreCustomer(a, svcByOwner[a.ID], txByService, st, asOf, fs, cs, false)
		if c.State == StateScored {
			c.Actions = actionResults(w, s.AsOf, svcByOwner[a.ID], txByService, st, asOf, a, c, funds, fs, cs, as)
		}
		customers[i] = c
	})
	if err != nil {
		return Result{}, err
	}
	res.Customers = customers
	for i, a := range accounts {
		c := customers[i]

		key := StateUnscored
		switch {
		case c.Level != nil:
			key = *c.Level
		case c.State == StateLost:
			key = StateLost
		}
		res.Summary.Levels[key]++
		res.Summary.ExpectedLossCents += c.ExpectedLossCents
		res.Summary.LostCents += c.LostCents
		if c.Level != nil && *c.Level != LevelLow {
			res.Summary.ExposureCents += c.ExposureCents
		}
		for _, sr := range c.Services {
			for _, q := range sr.Quality {
				res.Summary.QualityCounts[q]++
			}
			for _, id := range sr.QualityIDs {
				h := hits[id]
				h.OK = false
				h.Count++
				if len(h.Items) < maxCheckItems {
					h.Items = append(h.Items, CheckItem{ServiceID: sr.ID, AccountID: a.ID})
				}
			}
		}
		for _, sq := range c.Quality {
			for _, q := range sq.Flags {
				res.Summary.QualityCounts[q]++
			}
			for _, id := range sq.IDs {
				h := hits[id]
				h.OK = false
				h.Count++
				if len(h.Items) < maxCheckItems {
					h.Items = append(h.Items, CheckItem{ServiceID: sq.ServiceID, AccountID: a.ID})
				}
			}
		}
	}
	res.Summary.LossByFeature = lossByFeature(customers)
	for _, c := range cs {
		h := hits[c.ID()]
		h.Message = checkMessage(c, h.Count)
		res.Checks = append(res.Checks, *h)
	}
	return res, nil
}

// parallelFor вызывает fn(i) для i в [0, n) пулом воркеров. Индексы нарезаны на пачки: воркер берёт
// следующую пачку атомарным счётчиком, пишет результат в свою ячейку по индексу, общих блокировок нет.
// Между пачками проверяется ctx: после отмены новые пачки не берутся, возвращается ошибка контекста.
//
// groupBy раскладывает элементы по ключу с сохранением порядка. Группы — срезы одного общего массива:
// вместо append на каждый ключ — три выделения на весь индекс. Срез группы ограничен ёмкостью (cap = len),
// дописать в него нельзя, чужую группу он не затрёт.
func groupBy[T any](items []T, key func(T) int64) map[int64][]T {
	counts := make(map[int64]int)
	for _, it := range items {
		counts[key(it)]++
	}
	all := make([]T, len(items))
	out := make(map[int64][]T, len(counts))
	pos := 0
	for k, n := range counts {
		out[k] = all[pos : pos : pos+n]
		pos += n
	}
	for _, it := range items {
		k := key(it)
		out[k] = append(out[k], it)
	}
	return out
}

// scratch — буферы воркера для действий: живут весь прогон воркера и переиспользуются между его клиентами.
// В результат ничего из них не попадает.
func parallelFor(ctx context.Context, n int, opt Options, fn func(w *scratch, i int)) error {
	batch := opt.BatchSize
	if batch <= 0 {
		batch = DefaultBatchSize
	}
	run := func(w *scratch, lo, hi int) {
		var t0 time.Time
		if opt.OnBatch != nil {
			t0 = time.Now()
		}
		for i := lo; i < hi; i++ {
			fn(w, i)
		}
		if opt.OnBatch != nil {
			opt.OnBatch(hi-lo, time.Since(t0))
		}
	}
	workers := Workers(n)
	if workers <= 1 {
		w := &scratch{}
		for lo := 0; lo < n; lo += batch {
			if err := ctx.Err(); err != nil {
				return err
			}
			run(w, lo, min(lo+batch, n))
		}
		return nil
	}
	var next atomic.Int64
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := &scratch{}
			for ctx.Err() == nil {
				lo := int(next.Add(int64(batch))) - batch
				if lo >= n {
					return
				}
				run(w, lo, min(lo+batch, n))
			}
		}()
	}
	wg.Wait()
	return ctx.Err()
}

// scratch — буферы воркера для расчёта действий (данные одного клиента). Перезаписываются на каждом клиенте.
type scratch struct {
	acct [1]Account
	svcs []Service
	txs  []Transaction
}

// actionResults применяет каждое подходящее действие к копии снимка, где изменён только этот клиент.
// Копия содержит только данные клиента (аккаунт, его услуги и их платежи): действия и пересчёт
// клиента от остальных не зависят, а копировать весь снимок на каждое действие — квадратичная
// работа (на 10 000 аккаунтов пересчёт шёл минуту с лишним). Копия лежит в буферах воркера.
func actionResults(w *scratch, snapAsOf time.Time, svcs []Service, txByService map[int64][]Transaction, st Settings, asOf time.Time,
	a Account, c Customer, funds FundsCtx, fs []Feature, cs []Check, as []Action) []ActionResult {
	out := []ActionResult{}
	w.acct[0] = a
	w.svcs = append(w.svcs[:0], svcs...)
	w.txs = w.txs[:0]
	for _, sv := range w.svcs {
		w.txs = append(w.txs, txByService[sv.ID]...)
	}
	ctx := CustomerCtx{Account: a, Customer: c, Funds: funds, Services: w.svcs, Transactions: w.txs}
	s := Snapshot{AsOf: snapAsOf, Accounts: w.acct[:], Services: w.svcs, Transactions: w.txs}
	for _, act := range as {
		if !act.Applies(ctx) {
			continue
		}
		var applied Snapshot
		if sa, ok := act.(settingsAction); ok {
			applied = sa.ApplyWith(s, a.ID, st)
		} else {
			applied = act.Apply(s, a.ID)
		}
		after := scoreOne(applied, s, a.ID, txByService, st, asOf, fs, cs)
		out = append(out, ActionResult{
			ID:             act.ID(),
			Title:          act.Title(ctx),
			LossDeltaCents: after.ExpectedLossCents - c.ExpectedLossCents,
			ScoreBefore:    c.Score,
			ScoreAfter:     after.Score,
			LevelAfter:     after.Level,
		})
	}
	// первое — главное действие: наибольшее снижение потери (правило 9)
	slices.SortStableFunc(out, func(a, b ActionResult) int { return cmp.Compare(a.LossDeltaCents, b.LossDeltaCents) })
	return out
}

// sameTransactions — действие вернуло те же транзакции, что получило (не пересобрало срез).
func sameTransactions(a, b []Transaction) bool {
	return len(a) == len(b) && (len(a) == 0 || &a[0] == &b[0])
}

// scoreOne считает одного клиента без действий. Тексты причин не нужны: важны только потеря, скор и уровень.
// Если действие не тронуло транзакции, берётся индекс снимка (orig — снимок до действия, byService — его индекс).
func scoreOne(s, orig Snapshot, accountID int64, byService map[int64][]Transaction, st Settings, asOf time.Time, fs []Feature, cs []Check) Customer {
	txByService := byService
	if !sameTransactions(s.Transactions, orig.Transactions) {
		txByService = map[int64][]Transaction{}
		for _, t := range s.Transactions {
			txByService[t.ServiceID] = append(txByService[t.ServiceID], t)
		}
	}
	for _, a := range s.Accounts {
		if a.ID != accountID {
			continue
		}
		svcs := make([]Service, 0, len(s.Services))
		for _, sv := range s.Services {
			if sv.OwnerID == accountID {
				svcs = append(svcs, sv)
			}
		}
		c, _ := scoreCustomer(a, svcs, txByService, st, asOf, fs, cs, true)
		return c
	}
	return Customer{ID: accountID, State: StateUnscored}
}

func scoreCustomer(a Account, svcs []Service, txByService map[int64][]Transaction, st Settings, asOf time.Time,
	fs []Feature, cs []Check, noReason bool) (Customer, FundsCtx) {
	c := Customer{ID: a.ID, Actions: []ActionResult{}}
	if len(svcs) == 0 {
		c.State = StateUnscored
		return c, FundsCtx{}
	}
	slices.SortFunc(svcs, func(a, b Service) int { return cmp.Compare(a.ID, b.ID) })

	live := make([]Service, 0, len(svcs))
	for _, sv := range svcs {
		if sv.IsLive() {
			live = append(live, sv)
			continue
		}
		// проверки качества идут и по неживым услугам; признаков и скора у них нет
		h := qualityFlags(ServiceCtx{AsOf: asOf, Account: a, Service: sv, Transactions: txByService[sv.ID]}, cs)
		if len(h.Messages) > 0 {
			c.Quality = append(c.Quality, ServiceQuality{ServiceID: sv.ID, Flags: h.Messages, IDs: h.IDs})
		}
		asActive := sv
		asActive.Status = StatusActive
		c.LostCents += amount(asActive, asOf, st.horizonHours(), st.PriceBasis)
	}
	if len(live) == 0 {
		c.State = StateLost
		return c, FundsCtx{}
	}

	funds := accountFunds(a, live, asOf, st)
	best := 0.0
	c.Services = make([]ServiceResult, 0, len(live))
	for _, sv := range live {
		ctx := ServiceCtx{AsOf: asOf, Account: a, Service: sv, Transactions: txByService[sv.ID], Funds: funds, NoReason: noReason}
		sr := scoreService(ctx, st, fs, cs)
		c.Services = append(c.Services, sr)
		c.ExpectedLossCents += sr.LossCents
		c.ExposureCents += sr.RCents
		best = math.Max(best, sr.P)
	}
	c.State = StateScored
	score := round2(100 * best)
	level := LevelLow
	switch {
	case score >= st.High:
		level = LevelHigh
	case score >= st.Medium:
		level = LevelMedium
	}
	c.Score, c.Level = &score, &level
	return c, funds
}

func scoreService(ctx ServiceCtx, st Settings, fs []Feature, cs []Check) ServiceResult {
	sr := ServiceResult{ID: ctx.Service.ID, RCents: amount(ctx.Service, ctx.AsOf, st.horizonHours(), st.PriceBasis)}
	q := 1.0
	sr.Features = make([]FeatureHit, 0, len(fs))
	for _, f := range fs {
		v, reason, ok := f.Eval(ctx)
		if !ok {
			continue
		}
		v = math.Min(1, math.Max(0, v))
		w := st.weight(f)
		q *= 1 - w*v
		sr.Features = append(sr.Features, FeatureHit{ID: f.ID(), F: v, Weight: w, Contribution: w * v, Reason: reason})
	}
	if len(sr.Features) == 0 {
		sr.Features = nil
	}
	sr.P = 1 - q
	sr.LossCents = int64(math.RoundToEven(sr.P * float64(sr.RCents)))
	slices.SortStableFunc(sr.Features, func(a, b FeatureHit) int {
		if a.Contribution != b.Contribution {
			return cmp.Compare(b.Contribution, a.Contribution)
		}
		return cmp.Compare(a.ID, b.ID)
	})
	h := qualityFlags(ctx, cs)
	sr.Quality, sr.QualityIDs = h.Messages, h.IDs
	return sr
}

// qualityHits — сработавшие проверки услуги: тексты и id в одном порядке.
type qualityHits struct {
	Messages []string
	IDs      []string
}

func qualityFlags(ctx ServiceCtx, cs []Check) qualityHits {
	var out qualityHits
	for _, c := range cs {
		if msg, hit := c.Eval(ctx); hit {
			out.Messages = append(out.Messages, msg)
			out.IDs = append(out.IDs, c.ID())
		}
	}
	return out
}

// accountFunds — «нужно» и «есть» за окно запаса средств (правило 5, funds).
func accountFunds(a Account, live []Service, asOf time.Time, st Settings) FundsCtx {
	f := FundsCtx{Have: a.Balance}
	if a.BonusState == "unlocked" {
		f.Have += a.BonusBalance
	}
	for _, sv := range live {
		if sv.Status == StatusProlongWait {
			f.Need += sv.Price
		} else {
			f.Need += amount(sv, asOf, fundsHours, st.PriceBasis)
		}
	}
	return f
}

// round2 округляет до 2 знаков по точному двоичному значению (как round(x, 2) оракула).
func round2(x float64) float64 {
	v, _ := strconv.ParseFloat(strconv.FormatFloat(x, 'f', 2, 64), 64)
	return v
}

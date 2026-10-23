package risk

import (
	"math"
	"sort"
	"strconv"
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
	Actions           []ActionResult
}

// Summary — сводка. ExposureCents («под угрозой») — только клиенты high и medium.
type Summary struct {
	Levels            map[string]int
	ExpectedLossCents int64
	ExposureCents     int64
	LostCents         int64
	QualityCounts     map[string]int
}

// Result — результат расчёта по снимку.
type Result struct {
	AsOf      time.Time
	Customers []Customer
	Summary   Summary
}

// Score считает модель по правилам docs/model-rules.md. Чистая функция: часов не читает, ввода-вывода нет.
func Score(s Snapshot, st Settings, asOf time.Time) Result {
	return scoreWith(s, st, asOf, features.all(), checks.all(), actions.all())
}

func scoreWith(s Snapshot, st Settings, asOf time.Time, fs []Feature, cs []Check, as []Action) Result {
	svcByOwner := map[int64][]Service{}
	for _, sv := range s.Services {
		svcByOwner[sv.OwnerID] = append(svcByOwner[sv.OwnerID], sv)
	}
	txByService := map[int64][]Transaction{}
	for _, t := range s.Transactions {
		txByService[t.ServiceID] = append(txByService[t.ServiceID], t)
	}

	accounts := append([]Account(nil), s.Accounts...)
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].ID < accounts[j].ID })

	res := Result{AsOf: asOf, Summary: Summary{
		Levels:        map[string]int{LevelHigh: 0, LevelMedium: 0, LevelLow: 0, StateLost: 0, StateUnscored: 0},
		QualityCounts: map[string]int{},
	}}
	for _, a := range accounts {
		c, funds := scoreCustomer(a, svcByOwner[a.ID], txByService, st, asOf, fs, cs)
		if c.State == StateScored {
			c.Actions = actionResults(s, st, asOf, a, c, funds, fs, cs, as)
		}
		res.Customers = append(res.Customers, c)

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
		}
	}
	return res
}

// actionResults применяет каждое подходящее действие к копии снимка, где изменён только этот клиент.
func actionResults(s Snapshot, st Settings, asOf time.Time, a Account, c Customer, funds FundsCtx,
	fs []Feature, cs []Check, as []Action) []ActionResult {
	out := []ActionResult{}
	ctx := CustomerCtx{Account: a, Customer: c, Funds: funds}
	for _, act := range as {
		if !act.Applies(ctx) {
			continue
		}
		after := scoreOne(act.Apply(s, a.ID), a.ID, st, asOf, fs, cs)
		out = append(out, ActionResult{
			ID:             act.ID(),
			Title:          act.Title(ctx),
			LossDeltaCents: after.ExpectedLossCents - c.ExpectedLossCents,
			ScoreBefore:    c.Score,
			ScoreAfter:     after.Score,
			LevelAfter:     after.Level,
		})
	}
	return out
}

// scoreOne считает одного клиента без действий.
func scoreOne(s Snapshot, accountID int64, st Settings, asOf time.Time, fs []Feature, cs []Check) Customer {
	txByService := map[int64][]Transaction{}
	for _, t := range s.Transactions {
		txByService[t.ServiceID] = append(txByService[t.ServiceID], t)
	}
	for _, a := range s.Accounts {
		if a.ID != accountID {
			continue
		}
		var svcs []Service
		for _, sv := range s.Services {
			if sv.OwnerID == accountID {
				svcs = append(svcs, sv)
			}
		}
		c, _ := scoreCustomer(a, svcs, txByService, st, asOf, fs, cs)
		return c
	}
	return Customer{ID: accountID, State: StateUnscored}
}

func scoreCustomer(a Account, svcs []Service, txByService map[int64][]Transaction, st Settings, asOf time.Time,
	fs []Feature, cs []Check) (Customer, FundsCtx) {
	c := Customer{ID: a.ID, Actions: []ActionResult{}}
	if len(svcs) == 0 {
		c.State = StateUnscored
		return c, FundsCtx{}
	}
	sort.Slice(svcs, func(i, j int) bool { return svcs[i].ID < svcs[j].ID })

	var live []Service
	for _, sv := range svcs {
		if sv.IsLive() {
			live = append(live, sv)
			continue
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
	for _, sv := range live {
		ctx := ServiceCtx{AsOf: asOf, Account: a, Service: sv, Transactions: txByService[sv.ID], Funds: funds}
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
	sr.P = 1 - q
	sr.LossCents = int64(math.RoundToEven(sr.P * float64(sr.RCents)))
	sort.SliceStable(sr.Features, func(i, j int) bool {
		if sr.Features[i].Contribution != sr.Features[j].Contribution {
			return sr.Features[i].Contribution > sr.Features[j].Contribution
		}
		return sr.Features[i].ID < sr.Features[j].ID
	})
	for _, c := range cs {
		if msg, hit := c.Eval(ctx); hit {
			sr.Quality = append(sr.Quality, msg)
		}
	}
	return sr
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

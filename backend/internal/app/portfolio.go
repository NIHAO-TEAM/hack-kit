package app

import (
	"sort"
	"sync"
	"sync/atomic"

	"example.com/app/internal/analytics"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/risk"
)

// snapshots помнит снимки, по которым считал воркер: Engine хранит только результат, а для
// выдачи клиентов нужны email и деньги. Держим два последних, чтобы читатель, взявший
// результат до смены версии данных, нашёл свой снимок.
type snapshots struct {
	mu   sync.Mutex
	list []snapEntry
}

type snapEntry struct {
	version int64
	snap    risk.Snapshot
}

func (s *snapshots) put(version int64, snap risk.Snapshot) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.list {
		if s.list[i].version == version {
			s.list[i].snap = snap
			return
		}
	}
	s.list = append(s.list, snapEntry{version, snap})
	if len(s.list) > 2 {
		s.list = s.list[len(s.list)-2:]
	}
}

func (s *snapshots) get(version int64) (risk.Snapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range s.list {
		if e.version == version {
			return e.snap, true
		}
	}
	return risk.Snapshot{}, false
}

type portfolioCache struct {
	version int64
	pf      *analytics.Portfolio
}

// Portfolio отдаёт клиентов последнего пересчёта; nil, пока расчёта нет. Сборка идёт один раз
// на версию пересчёта.
func (r *Recalc) Portfolio() *analytics.Portfolio {
	calc := r.Engine.Latest()
	if calc == nil {
		return nil
	}
	if c := r.cache.Load(); c != nil && c.version == calc.Version {
		return c.pf
	}
	snap, ok := r.snaps.get(calc.DataVersion)
	if !ok {
		return nil
	}
	pf := buildPortfolio(calc.Result, snap, calc.Version)
	r.cache.Store(&portfolioCache{calc.Version, pf})
	return pf
}

var _ analytics.Feed = (*Recalc)(nil)

type cacheHolder = atomic.Pointer[portfolioCache]

func buildPortfolio(res risk.Result, snap risk.Snapshot, version int64) *analytics.Portfolio {
	accounts := map[int64]risk.Account{}
	for _, a := range snap.Accounts {
		accounts[a.ID] = a
	}
	services := map[int64][]risk.Service{}
	for _, sv := range snap.Services {
		services[sv.OwnerID] = append(services[sv.OwnerID], sv)
	}
	pf := &analytics.Portfolio{Version: version, Customers: make([]analytics.Customer, 0, len(res.Customers))}
	for _, c := range res.Customers {
		pf.Customers = append(pf.Customers, buildCustomer(c, accounts[c.ID], services[c.ID]))
	}
	sort.Slice(pf.Customers, func(i, j int) bool { return pf.Customers[i].Row.Id < pf.Customers[j].Row.Id })
	return pf
}

func buildCustomer(c risk.Customer, a risk.Account, svcs []risk.Service) analytics.Customer {
	level := gen.RiskLevel(c.State)
	if c.Level != nil {
		level = gen.RiskLevel(*c.Level)
	}
	out := analytics.Customer{
		Row: gen.CustomerRow{
			Id: c.ID, Email: a.Email, Balance: a.Balance, TotalReplenished: a.TotalReplenished,
			Score: c.Score, Level: level, ExpectedLossCents: c.ExpectedLossCents,
		},
		Account: gen.Account{
			Id: a.ID, Email: a.Email, Balance: a.Balance, WithdrawBalance: a.WithdrawBalance,
			TotalReplenished: a.TotalReplenished, BonusBalance: a.BonusBalance,
			BonusState: gen.AccountBonusState(a.BonusState),
		},
		Services: []gen.Service{},
		Risks:    []gen.ServiceRisk{},
		Features: []gen.Feature{},
		Actions:  []gen.Action{},
	}
	sort.Slice(svcs, func(i, j int) bool { return svcs[i].ID < svcs[j].ID })
	for _, sv := range svcs {
		out.Services = append(out.Services, gen.Service{
			Id: sv.ID, Name: sv.Name, OwnerId: sv.OwnerID, PaymentTerm: gen.PaymentTerm(sv.PaymentTerm),
			AutoProlong: sv.AutoProlong, CreatedAt: sv.CreatedAt, ExpiresAt: sv.ExpiresAt,
			Status: gen.ServiceStatus(sv.Status), Price: sv.Price,
		})
	}

	top := -1
	for i, sr := range c.Services {
		out.Risks = append(out.Risks, gen.ServiceRisk{
			ServiceId: sr.ID, Probability: sr.P, RCents: sr.RCents, LossCents: sr.LossCents,
			Features: featuresOf(sr.Features), Quality: append([]string{}, sr.Quality...),
		})
		if top < 0 || sr.P > c.Services[top].P {
			top = i
		}
	}
	if top >= 0 {
		out.Features = out.Risks[top].Features
		if len(out.Features) > 0 {
			reason := out.Features[0].Reason
			out.Row.TopReason = &reason
		}
	}
	for _, ar := range c.Actions {
		act := gen.Action{Id: ar.ID, Title: ar.Title, EffectCents: ar.LossDeltaCents}
		if ar.ScoreAfter != nil {
			act.NewScore = *ar.ScoreAfter
		}
		if ar.LevelAfter != nil {
			act.NewLevel = gen.RiskLevel(*ar.LevelAfter)
		}
		out.Actions = append(out.Actions, act)
		if ar.ID == "to_billing" {
			t := true
			out.Row.Billing = &t
		}
	}
	if len(out.Actions) > 0 {
		out.Row.TopAction = &gen.ActionBrief{
			Id: out.Actions[0].Id, Title: out.Actions[0].Title, EffectCents: out.Actions[0].EffectCents,
		}
	}
	// окно спасения: ближайшая к удалению услуга в ожидании оплаты
	var wait *risk.Service
	for i := range svcs {
		if svcs[i].Status == risk.StatusProlongWait && (wait == nil || svcs[i].ExpiresAt.Before(wait.ExpiresAt)) {
			wait = &svcs[i]
		}
	}
	if wait != nil {
		exp, del := wait.ExpiresAt, wait.ExpiresAt.Add(analytics.Window)
		out.Row.ExpiresAt, out.Row.DeleteAt = &exp, &del
	}
	return out
}

func featuresOf(hits []risk.FeatureHit) []gen.Feature {
	out := make([]gen.Feature, 0, len(hits))
	for _, h := range hits {
		out = append(out, gen.Feature{Id: h.ID, Value: h.F, Weight: h.Weight, Contribution: h.Contribution, Reason: h.Reason})
	}
	return out
}

// Package backtest проверяет ранжирование модели на прокрутке времени: скор на момент T,
// 30 суток поведения клиентов (behavior.json) и правил биллинга (source.Advance), исход по каждой
// живой услуге. Данных об оттоке нет (ADR-0001), поэтому это симуляция по нашим допущениям,
// а не калибровка вероятности и не данные aéza. Модель и веса не меняются.
package backtest

import (
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"time"

	"example.com/app/internal/risk"
	"example.com/app/internal/source"
)

// Horizon — сколько суток прокручиваем; совпадает с горизонтом денег модели (30 суток).
const Horizon = 30

// RandomRuns — по скольким зёрнам усредняется случайный порядок.
const RandomRuns = 20

// Названия способов ранжирования.
const (
	Model        = "model"
	ProlongWait  = "prolong_wait"
	NoAutoprolng = "no_autoprolong"
	LowFunds     = "low_funds"
	Price        = "price"
	Random       = "random"
)

// Methods — порядок способов в отчёте.
var Methods = []string{Model, ProlongWait, NoAutoprolng, LowFunds, Price, Random}

// Metrics — качество одного способа ранжирования.
type Metrics struct {
	// Top10 и Top20 — доля реально потерянных денег у первых 10 % и 20 % клиентов очереди.
	Top10, Top20 float64
	// ROC и PR — ROC AUC и PR AUC по услугам (скор против исхода).
	ROC, PR float64
}

// Result — итог одной прокрутки.
type Result struct {
	Customers, Services int
	LostServices        int
	LostCents, RCents   int64
	ByMethod            map[string]Metrics
}

// svc — живая услуга на момент T с признаками, доступными правилам.
type svc struct {
	id, owner int64
	p         float64
	r         int64
	waiting   bool
	noAuto    bool
	lowFunds  bool // на балансе меньше цены услуги
}

type cust struct {
	id       int64
	loss     int64 // ожидаемая потеря модели
	exposure int64
	waiting  int
	noAuto   int
	shortage int64 // сколько не хватает до суммы цен живых услуг
}

// Prepared — снимок, посчитанный на момент T: скор не зависит от поведения, считается один раз.
type Prepared struct {
	snap  source.Snapshot
	asOf  time.Time
	svcs  []svc
	custs []cust
	svcAt map[int64]int
}

// Prepare считает скор снимка с настройками по умолчанию.
func Prepare(s source.Snapshot) (*Prepared, error) {
	if s.AsOf == nil {
		return nil, fmt.Errorf("в снимке нет asOf")
	}
	asOf := s.AsOf.UTC()
	res := risk.Score(toRisk(s), risk.Defaults(), asOf)
	p := &Prepared{snap: s, asOf: asOf, svcAt: map[int64]int{}}
	byID := map[int64]source.Service{}
	for _, v := range s.Services {
		byID[v.ID] = v
	}
	bal := map[int64]int64{}
	for _, a := range s.Accounts {
		bal[a.ID] = a.Balance
	}
	for _, c := range res.Customers {
		if len(c.Services) == 0 {
			continue
		}
		cu := cust{id: c.ID, loss: c.ExpectedLossCents, exposure: c.ExposureCents}
		var sumPrice int64
		for _, sr := range c.Services {
			v := byID[sr.ID]
			sv := svc{id: sr.ID, owner: c.ID, p: sr.P, r: sr.RCents,
				waiting: v.Status == "prolong_wait", noAuto: !v.AutoProlong, lowFunds: bal[c.ID] < v.Price}
			p.svcAt[sv.id] = len(p.svcs)
			p.svcs = append(p.svcs, sv)
			sumPrice += v.Price
			if sv.waiting {
				cu.waiting++
			}
			if sv.noAuto {
				cu.noAuto++
			}
		}
		if d := sumPrice - bal[c.ID]; d > 0 {
			cu.shortage = d
		}
		p.custs = append(p.custs, cu)
	}
	return p, nil
}

// Run прокручивает 30 суток с поведением b и зерном seed и считает метрики всех способов.
func (p *Prepared) Run(b Behavior, seed uint64) Result {
	lost := p.outcomes(b, seed)
	return p.evaluate(lost, seed)
}

// outcomes — для каждой услуги из p.svcs: удалена к T+30.
func (p *Prepared) outcomes(b Behavior, seed uint64) []bool {
	rng := rand.New(rand.NewPCG(seed, 0x6261636b))
	// транзакции Advance не нужны; не копируем их 30 раз
	s := source.Snapshot{AsOf: p.snap.AsOf, Accounts: p.snap.Accounts, Services: p.snap.Services}
	for d := 0; d < Horizon; d++ {
		s.Accounts = append([]source.Account(nil), s.Accounts...)
		s.Services = append([]source.Service(nil), s.Services...)
		behave(&s, b, rng)
		s = source.Advance(s, 24)
	}
	status := make(map[int64]string, len(s.Services))
	for _, v := range s.Services {
		status[v.ID] = v.Status
	}
	out := make([]bool, len(p.svcs))
	for i, sv := range p.svcs {
		out[i] = status[sv.id] == "deleted"
	}
	return out
}

func (p *Prepared) evaluate(lost []bool, seed uint64) Result {
	r := Result{Customers: len(p.custs), Services: len(p.svcs), ByMethod: map[string]Metrics{}}
	lostBy := map[int64]int64{}
	for i, sv := range p.svcs {
		r.RCents += sv.r
		if lost[i] {
			r.LostServices++
			r.LostCents += sv.r
			lostBy[sv.owner] += sv.r
		}
	}
	custLost := make([]int64, len(p.custs))
	custIDs := make([]int64, len(p.custs))
	for i, c := range p.custs {
		custLost[i] = lostBy[c.id]
		custIDs[i] = c.id
	}
	svcKeys := map[string][]float64{}
	custKeys := map[string][]float64{}
	for _, m := range Methods {
		if m == Random {
			continue
		}
		svcKeys[m] = make([]float64, len(p.svcs))
		custKeys[m] = make([]float64, len(p.custs))
	}
	for i, sv := range p.svcs {
		svcKeys[Model][i] = sv.p
		svcKeys[ProlongWait][i] = b2f(sv.waiting)
		svcKeys[NoAutoprolng][i] = b2f(sv.noAuto)
		svcKeys[LowFunds][i] = b2f(sv.lowFunds)
		svcKeys[Price][i] = float64(sv.r)
	}
	for i, c := range p.custs {
		custKeys[Model][i] = float64(c.loss)
		custKeys[ProlongWait][i] = float64(c.waiting)
		custKeys[NoAutoprolng][i] = float64(c.noAuto)
		custKeys[LowFunds][i] = float64(c.shortage)
		custKeys[Price][i] = float64(c.exposure)
	}
	for m, sk := range svcKeys {
		r.ByMethod[m] = metrics(sk, lost, custKeys[m], custIDs, custLost)
	}
	var acc Metrics
	for k := 0; k < RandomRuns; k++ {
		rng := rand.New(rand.NewPCG(seed+uint64(k)+1, 0x72616e64))
		sk := make([]float64, len(p.svcs))
		ck := make([]float64, len(p.custs))
		for i := range sk {
			sk[i] = rng.Float64()
		}
		for i := range ck {
			ck[i] = rng.Float64()
		}
		m := metrics(sk, lost, ck, custIDs, custLost)
		acc.Top10 += m.Top10 / RandomRuns
		acc.Top20 += m.Top20 / RandomRuns
		acc.ROC += m.ROC / RandomRuns
		acc.PR += m.PR / RandomRuns
	}
	r.ByMethod[Random] = acc
	return r
}

func metrics(svcKey []float64, lost []bool, custKey []float64, custIDs, custLost []int64) Metrics {
	ord := Order(custKey, custIDs)
	return Metrics{
		Top10: TopShare(ord, custLost, 0.10),
		Top20: TopShare(ord, custLost, 0.20),
		ROC:   ROCAUC(svcKey, lost),
		PR:    PRAUC(svcKey, lost),
	}
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func toRisk(s source.Snapshot) risk.Snapshot {
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

// Shift — результат устойчивости: все вероятности (Param пустой) или одна, сдвинутые на Factor.
type Shift struct {
	Param  string
	Factor float64
	Result Result
}

// Robustness прогоняет сдвиги каждой вероятности на −50 % и +50 %, затем всех сразу.
func (p *Prepared) Robustness(b Behavior, seed uint64) []Shift {
	var out []Shift
	for _, name := range append(paramNames(), "") {
		for _, k := range []float64{0.5, 1.5} {
			out = append(out, Shift{Param: name, Factor: k, Result: p.Run(b.Scaled(name, k), seed)})
		}
	}
	return out
}

func paramNames() []string {
	n := make([]string, len(Params))
	for i, p := range Params {
		n[i] = p.Name
	}
	return n
}

// BestRule — лучший из простых правил (без модели и случайного порядка) по выбранной метрике.
func BestRule(r Result, pick func(Metrics) float64) (name string, v float64) {
	v = math.Inf(-1)
	names := []string{ProlongWait, NoAutoprolng, LowFunds, Price}
	sort.Strings(names)
	for _, m := range names {
		if x := pick(r.ByMethod[m]); x > v {
			name, v = m, x
		}
	}
	return name, v
}

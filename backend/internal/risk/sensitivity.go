package risk

import (
	"hash/fnv"
	"sort"
	"strconv"
	"time"
)

// Параметры проверки чувствительности: сдвиг веса и выборка для больших портфелей.
const (
	// SensitivityShift — на сколько сдвигается вес признака: −20 % и +20 %.
	SensitivityShift = 0.2
	// SensitivityMaxCustomers — больше стольких клиентов считаем на выборке.
	SensitivityMaxCustomers = 20000
	// SensitivitySampleSize — размер выборки.
	SensitivitySampleSize = 5000
	// topN — размер «верха очереди».
	topN = 3
)

// Shift — итог одного сдвига веса относительно базового расчёта.
type Shift struct {
	LevelChanges    int
	TopThreeChanged bool
	LossDeltaCents  int64
}

// FeatureSensitivity — чувствительность к весу одного признака.
type FeatureSensitivity struct {
	ID     string
	Weight float64
	Down   Shift
	Up     Shift
}

// SensitivityResult — чувствительность очереди к весам. Customers — сколько клиентов в расчёте
// (размер выборки, если Sampled).
type SensitivityResult struct {
	Features  []FeatureSensitivity
	Sampled   bool
	Customers int
}

// ShiftWeight возвращает копию настроек, где вес признака id умножен на factor и ограничен [0, 1].
// Исходные настройки не меняются.
func ShiftWeight(st Settings, id string, factor float64) Settings {
	out := st
	out.Weights = make(map[string]float64, len(st.Weights))
	for k, v := range st.Weights {
		out.Weights[k] = v
	}
	base, ok := out.Weights[id]
	if !ok {
		for _, f := range features.all() {
			if f.ID() == id {
				base = f.DefaultWeight()
			}
		}
	}
	out.Weights[id] = clamp01(base * factor)
	return out
}

// Sensitivity сдвигает вес каждого признака с ненулевым весом на −20 % и +20 % и сравнивает
// с базой: сколько клиентов сменили уровень, изменилась ли первая тройка по потере, на сколько
// сдвинулась суммарная потеря. Те же функции, что у движка; действия и проверки не считаются.
// Клиентов больше SensitivityMaxCustomers — расчёт на детерминированной выборке по хэшу id.
func Sensitivity(s Snapshot, st Settings, asOf time.Time) SensitivityResult {
	s, sampled := sampleSnapshot(s)
	base := scoreWith(s, st, asOf, features.all(), nil, nil)
	res := SensitivityResult{Features: []FeatureSensitivity{}, Sampled: sampled, Customers: len(s.Accounts)}
	baseTop := topIDs(base)
	baseLevels := levelsByID(base)
	for _, f := range features.all() {
		w := st.weight(f)
		if w == 0 {
			continue
		}
		shift := func(factor float64) Shift {
			next := ShiftWeight(st, f.ID(), factor)
			r := scoreWith(s, next, asOf, features.all(), nil, nil)
			sh := Shift{LossDeltaCents: r.Summary.ExpectedLossCents - base.Summary.ExpectedLossCents}
			for _, c := range r.Customers {
				if c.Level != nil && baseLevels[c.ID] != *c.Level {
					sh.LevelChanges++
				}
			}
			sh.TopThreeChanged = !sameIDs(baseTop, topIDs(r))
			return sh
		}
		res.Features = append(res.Features, FeatureSensitivity{
			ID: f.ID(), Weight: w, Down: shift(1 - SensitivityShift), Up: shift(1 + SensitivityShift),
		})
	}
	sort.SliceStable(res.Features, func(i, j int) bool { return res.Features[i].ID < res.Features[j].ID })
	return res
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func levelsByID(r Result) map[int64]string {
	m := make(map[int64]string, len(r.Customers))
	for _, c := range r.Customers {
		if c.Level != nil {
			m[c.ID] = *c.Level
		}
	}
	return m
}

// topIDs — id первых topN клиентов по ожидаемой потере (при равенстве — по id).
func topIDs(r Result) []int64 {
	cs := make([]Customer, 0, len(r.Customers))
	for _, c := range r.Customers {
		if c.State == StateScored {
			cs = append(cs, c)
		}
	}
	sort.SliceStable(cs, func(i, j int) bool {
		if cs[i].ExpectedLossCents != cs[j].ExpectedLossCents {
			return cs[i].ExpectedLossCents > cs[j].ExpectedLossCents
		}
		return cs[i].ID < cs[j].ID
	})
	if len(cs) > topN {
		cs = cs[:topN]
	}
	ids := make([]int64, len(cs))
	for i, c := range cs {
		ids[i] = c.ID
	}
	return ids
}

func sameIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// sampleSnapshot оставляет SensitivitySampleSize аккаунтов с наименьшим хэшем id, если их больше
// SensitivityMaxCustomers: выборка не зависит от порядка и повторяется от запроса к запросу.
func sampleSnapshot(s Snapshot) (Snapshot, bool) {
	if len(s.Accounts) <= SensitivityMaxCustomers {
		return s, false
	}
	type keyed struct {
		a Account
		h uint64
	}
	ks := make([]keyed, len(s.Accounts))
	for i, a := range s.Accounts {
		h := fnv.New64a()
		_, _ = h.Write([]byte(strconv.FormatInt(a.ID, 10)))
		ks[i] = keyed{a, h.Sum64()}
	}
	sort.Slice(ks, func(i, j int) bool {
		if ks[i].h != ks[j].h {
			return ks[i].h < ks[j].h
		}
		return ks[i].a.ID < ks[j].a.ID
	})
	ks = ks[:SensitivitySampleSize]
	keep := make(map[int64]bool, len(ks))
	out := Snapshot{AsOf: s.AsOf}
	for _, k := range ks {
		keep[k.a.ID] = true
		out.Accounts = append(out.Accounts, k.a)
	}
	svc := map[int64]bool{}
	for _, sv := range s.Services {
		if keep[sv.OwnerID] {
			svc[sv.ID] = true
			out.Services = append(out.Services, sv)
		}
	}
	for _, t := range s.Transactions {
		if svc[t.ServiceID] {
			out.Transactions = append(out.Transactions, t)
		}
	}
	return out, true
}

package analytics_test

import (
	"math/rand"
	"slices"
	"testing"

	"example.com/app/internal/analytics"
	"example.com/app/internal/platform/api/gen"
)

func byID(pf *analytics.Portfolio) {
	slices.SortFunc(pf.Customers, func(a, b analytics.Customer) int { return int(a.Row.Id - b.Row.Id) })
}

// 2.3: модель чтения после замены строк даёт тот же порядок и те же строки, что построенная с нуля;
// прежний портфель при этом не меняется.
func TestPatchedPortfolioMatchesFresh(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	levels := []gen.RiskLevel{gen.High, gen.Medium, gen.Low, gen.Unscored, gen.Lost}
	for round := 0; round < 20; round++ {
		prev := tiedPortfolio(500)
		byID(prev)
		prev.BuildIndex()
		fresh := &analytics.Portfolio{Version: 9, Customers: slices.Clone(prev.Customers)}

		var changed []analytics.Customer
		for k := 1 + rng.Intn(40); k > 0; k-- {
			c := prev.Customers[rng.Intn(len(prev.Customers))]
			c.Row.Balance = int64(rng.Intn(6))
			c.Row.TotalReplenished = int64(rng.Intn(4))
			c.Row.Email = []string{"A", "b", "Zed", "a"}[rng.Intn(4)] + string(rune('a'+rng.Intn(7))) + "@example.test"
			c.Row.Level = levels[rng.Intn(len(levels))]
			if c.Row.Level == gen.Unscored || c.Row.Level == gen.Lost {
				c.Row.Score, c.Row.ExpectedLossCents = nil, 0
			} else {
				s := float64(rng.Intn(11))
				c.Row.Score, c.Row.ExpectedLossCents = &s, int64(rng.Intn(13))
			}
			changed = append(changed, c)
			fresh.Customers[slices.IndexFunc(fresh.Customers, func(x analytics.Customer) bool { return x.Row.Id == c.Row.Id })] = c
		}
		patched := analytics.PatchPortfolio(prev, 9, changed)
		if patched == nil || patched.Version != 9 {
			t.Fatalf("раунд %d: портфель %+v", round, patched)
		}

		high := gen.High
		for _, sortBy := range []gen.ListCustomersParamsSort{"id", "email", "balance", "totalReplenished", "score", "expectedLoss"} {
			for _, order := range []gen.ListCustomersParamsOrder{gen.Asc, gen.Desc} {
				for _, level := range []*gen.RiskLevel{nil, &high} {
					p := gen.ListCustomersParams{Sort: &sortBy, Order: &order, Level: level, Limit: ptr(200)}
					got, code := list(t, listAPI(patched), p)
					want, _ := list(t, listAPI(fresh), p)
					if code != 200 || !slices.EqualFunc(got.Items, want.Items, func(a, b gen.CustomerRow) bool {
						return a.Id == b.Id && a.Email == b.Email && a.Balance == b.Balance && a.Level == b.Level
					}) || got.Total != want.Total {
						t.Fatalf("раунд %d, %s %s: порядок или строки не совпали со сборкой с нуля", round, sortBy, order)
					}
				}
			}
		}
		// прежний портфель остался прежним
		old := tiedPortfolio(500)
		byID(old)
		sortBy := gen.ListCustomersParamsSort("email")
		asc := gen.Asc
		got, _ := list(t, listAPI(prev), gen.ListCustomersParams{Sort: &sortBy, Order: &asc, Limit: ptr(200)})
		want := reference(old, nil, sortBy, false)
		if !slices.Equal(ids(got.Items), want[:len(got.Items)]) {
			t.Fatalf("раунд %d: прежний портфель изменился после PatchPortfolio", round)
		}
	}
}

func TestPatchPortfolioUnknownCustomer(t *testing.T) {
	prev := tiedPortfolio(20)
	byID(prev)
	if pf := analytics.PatchPortfolio(prev, 2, []analytics.Customer{{Row: gen.CustomerRow{Id: 999}}}); pf != nil {
		t.Error("клиента нет в прежнем портфеле: ждали nil")
	}
}

// 2.3: обход курсором по портфелю после замены строк идёт без повторов и пропусков в том же порядке.
func TestCursorWalksPatchedPortfolio(t *testing.T) {
	prev := tiedPortfolio(2_000)
	byID(prev)
	prev.BuildIndex()
	var changed []analytics.Customer
	for i := 0; i < 2_000; i += 37 {
		c := prev.Customers[i]
		c.Row.ExpectedLossCents += 5
		c.Row.Email = "zz" + c.Row.Email
		changed = append(changed, c)
	}
	pf := analytics.PatchPortfolio(prev, 5, changed)
	api := listAPI(pf)
	for _, sortBy := range []gen.ListCustomersParamsSort{"expectedLoss", "email"} {
		want := reference(pf, nil, sortBy, true)
		var seen []int64
		var cur *string
		for pages := 0; pages < 100; pages++ {
			got, code := list(t, api, gen.ListCustomersParams{Sort: &sortBy, Limit: ptr(150), Cursor: cur})
			if code != 200 {
				t.Fatalf("статус %d", code)
			}
			seen = append(seen, ids(got.Items)...)
			if cur = got.NextCursor; cur == nil {
				break
			}
		}
		if !slices.Equal(seen, want) {
			t.Errorf("%s: обход курсором (%d строк) не совпал с порядком (%d)", sortBy, len(seen), len(want))
		}
	}
}

// Фильтр по причине после PatchPortfolio совпадает со сборкой с нуля: причины сохранены, индекс пересчитан
// (потеря по причине у клиента появилась, исчезла и изменилась).
func TestPatchedPortfolioReasonMatchesFresh(t *testing.T) {
	mk := func() *analytics.Portfolio {
		pf := tiedPortfolio(60)
		byID(pf)
		pf.Reasons = []string{"susp", "debt"}
		for i := range pf.Customers {
			if i%3 == 0 {
				pf.Customers[i].Loss = map[string]int64{"susp": int64(100 + i)}
			}
		}
		return pf
	}
	prev := mk()
	prev.BuildIndex()
	fresh := mk()
	var changed []analytics.Customer
	set := func(i int, loss map[string]int64) {
		c := prev.Customers[i]
		c.Loss = loss
		changed = append(changed, c)
		fresh.Customers[i] = c
	}
	set(0, nil)                                    // исчезла
	set(1, map[string]int64{"susp": 5})            // появилась
	set(3, map[string]int64{"susp": 9999})         // изменилась
	set(4, map[string]int64{"debt": 7, "susp": 1}) // новая причина
	patched := analytics.PatchPortfolio(prev, 9, changed)
	if patched == nil {
		t.Fatal("PatchPortfolio вернул nil")
	}
	for _, reason := range []string{"susp", "debt"} {
		for _, order := range []gen.ListCustomersParamsOrder{gen.Asc, gen.Desc} {
			r := reason
			p := gen.ListCustomersParams{Reason: &r, Order: &order, Limit: ptr(200)}
			got, code := list(t, listAPI(patched), p)
			want, wcode := list(t, listAPI(fresh), p)
			if code != 200 || wcode != 200 || got.Total != want.Total || !slices.Equal(ids(got.Items), ids(want.Items)) {
				t.Fatalf("reason=%s %s: код %d, получили %v, ждали %v", reason, order, code, ids(got.Items), ids(want.Items))
			}
			if reason == "susp" && got.Total == 0 {
				t.Fatalf("reason=susp: пусто")
			}
		}
	}
}

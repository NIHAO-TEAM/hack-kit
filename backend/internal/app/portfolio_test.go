package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"example.com/app/internal/analytics"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/risk"
)

type feed struct{ pf *analytics.Portfolio }

func (f feed) Portfolio() *analytics.Portfolio { return f.pf }

// organizersPortfolio — данные организаторов через ту же сборку, что и сервер.
func organizersPortfolio(t *testing.T) *analytics.Portfolio {
	t.Helper()
	raw, err := os.ReadFile("../../../cases/analizator/data.json")
	if err != nil {
		t.Fatal(err)
	}
	var snap risk.Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	asOf := time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC)
	snap.AsOf = asOf
	return buildPortfolio(risk.Score(snap, risk.Defaults(), asOf), snap, 7)
}

func api(pf *analytics.Portfolio) *analytics.AnalyticsAPI {
	return analytics.New(nil, events.New(), clock.NewFake(time.Now())).WithFeed(feed{pf})
}

func list(t *testing.T, a *analytics.AnalyticsAPI, p gen.ListCustomersParams) gen.CustomerPage {
	t.Helper()
	w := httptest.NewRecorder()
	a.ListCustomers(w, httptest.NewRequest("GET", "/api/v1/customers", nil), p)
	if w.Code != http.StatusOK {
		t.Fatalf("статус %d: %s", w.Code, w.Body)
	}
	var out gen.CustomerPage
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func ids(p gen.CustomerPage) []int64 {
	var out []int64
	for _, r := range p.Items {
		out = append(out, r.Id)
	}
	return out
}

func TestListCustomersDefault(t *testing.T) {
	page := list(t, api(organizersPortfolio(t)), gen.ListCustomersParams{})
	if page.Total != 10 || page.Version != 7 {
		t.Fatalf("total=%d version=%d", page.Total, page.Version)
	}
	if got := ids(page); got[0] != 9 || got[1] != 1 {
		t.Errorf("первые строки %v, ждали 9 и 1", got)
	}
	if page.Items[0].ExpectedLossCents != 345927 || page.Items[1].ExpectedLossCents != 38315 {
		t.Errorf("потери %d и %d", page.Items[0].ExpectedLossCents, page.Items[1].ExpectedLossCents)
	}
	tail := page.Items[len(page.Items)-4:]
	for i, want := range []int64{2, 3, 7, 8} {
		if tail[i].Id != want || tail[i].Level != gen.Lost {
			t.Errorf("хвост[%d] = %d %s, ждали %d lost", i, tail[i].Id, tail[i].Level, want)
		}
	}
}

func TestListCustomersSorts(t *testing.T) {
	a := api(organizersPortfolio(t))
	for _, field := range []gen.ListCustomersParamsSort{gen.Id, gen.Email, gen.Balance, "totalReplenished", gen.Score, gen.ExpectedLoss} {
		for _, order := range []gen.ListCustomersParamsOrder{gen.Asc, gen.Desc} {
			f, o := field, order
			page := list(t, a, gen.ListCustomersParams{Sort: &f, Order: &o})
			seenBottom := false
			var prev *gen.CustomerRow
			for i := range page.Items {
				r := page.Items[i]
				if r.Level == gen.Lost || r.Level == gen.Unscored {
					seenBottom = true
					continue
				}
				if seenBottom {
					t.Fatalf("%s %s: оценённый клиент %d после «потерян»", f, o, r.Id)
				}
				if prev != nil && !ordered(f, o, *prev, r) {
					t.Errorf("%s %s: %d перед %d нарушает порядок", f, o, prev.Id, r.Id)
				}
				prev = &page.Items[i]
			}
		}
	}
}

func ordered(f gen.ListCustomersParamsSort, o gen.ListCustomersParamsOrder, x, y gen.CustomerRow) bool {
	var c int
	switch f {
	case gen.Id:
		c = cmpI(x.Id, y.Id)
	case gen.Email:
		c = cmpS(x.Email, y.Email)
	case gen.Balance:
		c = cmpI(x.Balance, y.Balance)
	case "totalReplenished":
		c = cmpI(x.TotalReplenished, y.TotalReplenished)
	case gen.Score:
		c = cmpF(*x.Score, *y.Score)
	default:
		c = cmpI(x.ExpectedLossCents, y.ExpectedLossCents)
	}
	if o == gen.Desc {
		return c >= 0
	}
	return c <= 0
}

func cmpI(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func cmpF(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func cmpS(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func TestListCustomersFilterAndPaging(t *testing.T) {
	a := api(organizersPortfolio(t))
	lost := gen.Lost
	page := list(t, a, gen.ListCustomersParams{Level: &lost})
	if page.Total != 4 {
		t.Errorf("lost: total=%d", page.Total)
	}
	one, off := 1, 1
	page = list(t, a, gen.ListCustomersParams{Limit: &one, Offset: &off})
	if page.Total != 10 || len(page.Items) != 1 || page.Items[0].Id != 1 {
		t.Errorf("страница: total=%d ids=%v", page.Total, ids(page))
	}
	q := "9"
	page = list(t, a, gen.ListCustomersParams{Q: &q})
	if page.Total == 0 || page.Items[0].Id != 9 {
		t.Errorf("поиск: %v", ids(page))
	}
}

func TestListCustomersBadParams(t *testing.T) {
	a := api(organizersPortfolio(t))
	zero, big := 0, 201
	for _, p := range []gen.ListCustomersParams{{Limit: &zero}, {Limit: &big}} {
		w := httptest.NewRecorder()
		a.ListCustomers(w, httptest.NewRequest("GET", "/api/v1/customers", nil), p)
		if w.Code != http.StatusBadRequest {
			t.Errorf("статус %d, ждали 400", w.Code)
		}
	}
}

func get(t *testing.T, a *analytics.AnalyticsAPI, id int64) (*httptest.ResponseRecorder, gen.CustomerDetail) {
	t.Helper()
	w := httptest.NewRecorder()
	a.GetCustomer(w, httptest.NewRequest("GET", "/api/v1/customers/1", nil), id)
	var d gen.CustomerDetail
	_ = json.Unmarshal(w.Body.Bytes(), &d)
	return w, d
}

func TestGetCustomer9(t *testing.T) {
	w, d := get(t, api(organizersPortfolio(t)), 9)
	if w.Code != 200 {
		t.Fatalf("статус %d", w.Code)
	}
	if d.Customer.ExpectedLossCents != 345927 {
		t.Errorf("потеря №9 = %d, ждали 345927", d.Customer.ExpectedLossCents)
	}
	if len(d.Actions) == 0 || d.Actions[0].Id != "top_up" || d.Actions[0].EffectCents != -106748 {
		t.Errorf("действия %+v, ждали пополнить −106748", d.Actions)
	}
	if d.Customer.TopAction == nil || d.Customer.TopAction.EffectCents != -106748 {
		t.Errorf("topAction %+v", d.Customer.TopAction)
	}
	if len(d.Services) == 0 || d.ServiceRisks == nil || len(*d.ServiceRisks) == 0 || len(d.Features) == 0 || d.Explanation == nil {
		t.Errorf("карточка неполная: %d услуг, %d оценок, %d признаков", len(d.Services), len(*d.ServiceRisks), len(d.Features))
	}
	if d.Version != 7 || d.Account == nil || d.Account.Email == "" {
		t.Errorf("version=%d account=%+v", d.Version, d.Account)
	}
}

func TestGetCustomerLostAndMissing(t *testing.T) {
	a := api(organizersPortfolio(t))
	w, d := get(t, a, 2)
	if w.Code != 200 || d.Customer.Level != gen.Lost || d.Customer.Score != nil {
		t.Errorf("№2: статус %d, %s, score %v", w.Code, d.Customer.Level, d.Customer.Score)
	}
	if d.Actions == nil || d.Features == nil || d.Services == nil {
		t.Error("пустые списки должны быть [], а не null")
	}
	if w, _ := get(t, a, 12345); w.Code != http.StatusNotFound {
		t.Errorf("неизвестный ID: статус %d", w.Code)
	}
}

func TestNoFeed(t *testing.T) {
	w := httptest.NewRecorder()
	analytics.New(nil, events.New(), clock.NewFake(time.Now())).ListCustomers(w, httptest.NewRequest("GET", "/", nil), gen.ListCustomersParams{})
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("статус %d, ждали 503", w.Code)
	}
}

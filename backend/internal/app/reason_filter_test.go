package app

import (
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/app/internal/platform/api/gen"
)

func strp(s string) *string { return &s }

// На данных организаторов (docs/model.md) по причине susp — 2 клиента, 1 844,19 € потери.
func TestListByReasonSusp(t *testing.T) {
	a := api(organizersPortfolio(t))
	page := list(t, a, gen.ListCustomersParams{Reason: strp("susp")})
	if page.Total != 2 || len(page.Items) != 2 {
		t.Fatalf("total=%d items=%d, нужно 2", page.Total, len(page.Items))
	}
	var sum int64
	for i, r := range page.Items {
		if r.ReasonLossCents == nil || *r.ReasonLossCents <= 0 {
			t.Fatalf("у клиента %d нет потери по причине", r.Id)
		}
		if i > 0 && *r.ReasonLossCents > *page.Items[i-1].ReasonLossCents {
			t.Fatalf("порядок не по убыванию потери по причине: %v", ids(page))
		}
		sum += *r.ReasonLossCents
	}
	if sum != 184419 {
		t.Fatalf("сумма по причине %d, в сводке 184419", sum)
	}
	asc := gen.Asc
	rev := list(t, a, gen.ListCustomersParams{Reason: strp("susp"), Order: &asc})
	if got, want := ids(rev), []int64{page.Items[1].Id, page.Items[0].Id}; got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("по возрастанию %v, ждали %v", got, want)
	}
}

func TestListByReasonKeepsOtherSortAndPages(t *testing.T) {
	a := api(organizersPortfolio(t))
	byID := gen.ListCustomersParamsSort("id")
	asc := gen.Asc
	one := 1
	p := gen.ListCustomersParams{Reason: strp("susp"), Sort: &byID, Order: &asc, Limit: &one}
	first := list(t, a, p)
	if first.Total != 2 || len(first.Items) != 1 || first.NextCursor == nil {
		t.Fatalf("первая страница: %+v", first)
	}
	if first.Items[0].ReasonLossCents == nil {
		t.Fatal("в строке нет потери по причине")
	}
	p.Cursor = first.NextCursor
	second := list(t, a, p)
	if len(second.Items) != 1 || second.Items[0].Id <= first.Items[0].Id {
		t.Fatalf("вторая страница: %v после %v", ids(second), ids(first))
	}
}

func TestListWithoutReasonHasNoReasonLoss(t *testing.T) {
	a := api(organizersPortfolio(t))
	one := 1
	for _, r := range list(t, a, gen.ListCustomersParams{Limit: &one}).Items {
		if r.ReasonLossCents != nil {
			t.Fatal("reasonLossCents без параметра reason")
		}
	}
}

func TestListUnknownReasonIs400(t *testing.T) {
	a := api(organizersPortfolio(t))
	w := httptest.NewRecorder()
	a.ListCustomers(w, httptest.NewRequest("GET", "/api/v1/customers", nil), gen.ListCustomersParams{Reason: strp("nope")})
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "susp") {
		t.Fatalf("статус %d: %s", w.Code, w.Body)
	}
}

func TestExportRespectsReason(t *testing.T) {
	a := api(organizersPortfolio(t))
	w := httptest.NewRecorder()
	a.ExportCustomers(w, httptest.NewRequest("GET", "/api/v1/customers/export.csv", nil), gen.ExportCustomersParams{Reason: strp("susp")})
	if w.Code != http.StatusOK {
		t.Fatalf("статус %d: %s", w.Code, w.Body)
	}
	cr := csv.NewReader(strings.NewReader(strings.TrimPrefix(w.Body.String(), "\xef\xbb\xbf")))
	cr.Comma = ';'
	rows, err := cr.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 { // заголовок и 2 клиента
		t.Fatalf("строк %d, нужно 3", len(rows))
	}
}

package app

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"example.com/app/internal/audit"
	"example.com/app/internal/platform/api/gen"
)

type memJournal struct {
	mu sync.Mutex
	es []audit.Entry
}

func (m *memJournal) Record(_ context.Context, e audit.Entry) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.es = append(m.es, e)
}

const bom = "\uFEFF"

func export(t *testing.T, p gen.ExportCustomersParams, j *memJournal) (*httptest.ResponseRecorder, [][]string) {
	t.Helper()
	a := api(organizersPortfolio(t))
	if j != nil {
		a.WithAudit(j)
	}
	w := httptest.NewRecorder()
	a.ExportCustomers(w, httptest.NewRequest("GET", "/api/v1/customers/export.csv", nil), p)
	if w.Code != http.StatusOK {
		return w, nil
	}
	body := w.Body.String()
	if !strings.HasPrefix(body, bom) {
		t.Fatalf("нет BOM: %q", body[:min(20, len(body))])
	}
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(body, bom)))
	r.Comma = ';'
	recs, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return w, recs
}

func TestExportCustomersOrganizers(t *testing.T) {
	w, recs := export(t, gen.ExportCustomersParams{}, nil)
	if len(recs) != 11 {
		t.Fatalf("строк %d, ждали 10 и заголовок", len(recs))
	}
	if recs[0][0] != "ID" || recs[1][0] != "9" {
		t.Errorf("заголовок %v, первая строка %v: ждали клиента 9 с наибольшей потерей", recs[0], recs[1])
	}
	if recs[1][6] != "3189,10" {
		t.Errorf("потеря клиента 9 = %q, ждали 3189,10", recs[1][6])
	}
	var sum int64
	for _, rec := range recs[1:] {
		var eur, cent int64
		if _, err := fmtSscan(rec[6], &eur, &cent); err != nil {
			t.Fatalf("потеря %q: %v", rec[6], err)
		}
		sum += eur*100 + cent
	}
	if sum != 359907 {
		t.Errorf("сумма потерь %d центов, ждали 359907", sum)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type %q", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="churn-queue-`) || !strings.HasSuffix(cd, `.csv"`) {
		t.Errorf("Content-Disposition %q", cd)
	}
}

func fmtSscan(s string, eur, cent *int64) (int, error) {
	return fmt.Sscanf(strings.Replace(s, ",", " ", 1), "%d %d", eur, cent)
}

func TestExportCustomersFilters(t *testing.T) {
	high := gen.High
	_, recs := export(t, gen.ExportCustomersParams{Level: &high}, nil)
	for _, rec := range recs[1:] {
		if rec[5] != "Высокий риск" {
			t.Errorf("строка %v не из фильтра level=high", rec)
		}
	}
	only, asc, sortID := "only", "asc", "id"
	_, recs = export(t, gen.ExportCustomersParams{Billing: &only}, nil)
	for _, rec := range recs[1:] {
		if rec[10] != "да" {
			t.Errorf("строка %v не «к биллингу»", rec)
		}
	}
	_, recs = export(t, gen.ExportCustomersParams{Sort: &sortID, Order: &asc}, nil)
	if recs[1][0] != "1" || recs[10][0] != "10" {
		t.Errorf("сортировка по id: %v … %v", recs[1][0], recs[10][0])
	}
	bad := "x"
	if w, _ := export(t, gen.ExportCustomersParams{Billing: &bad}, nil); w.Code != 400 {
		t.Errorf("billing=x: %d, ждали 400", w.Code)
	}
	if w, _ := export(t, gen.ExportCustomersParams{Order: &bad}, nil); w.Code != 400 {
		t.Errorf("order=x: %d, ждали 400", w.Code)
	}
}

func TestExportCustomersJournalHasNoEmail(t *testing.T) {
	j := &memJournal{}
	q := recsEmail(t)
	_, recs := export(t, gen.ExportCustomersParams{Q: &q}, j)
	if len(j.es) != 1 || j.es[0].Action != audit.ActionCustomersExported {
		t.Fatalf("журнал: %+v", j.es)
	}
	e := j.es[0]
	if !strings.Contains(e.Summary, "строк") || len(recs) < 2 {
		t.Errorf("запись %+v, строк в файле %d", e, len(recs))
	}
	all := e.Actor + e.Summary
	for _, c := range e.Changes {
		all += c.Field + c.From + c.To
	}
	if strings.Contains(all, q) {
		t.Errorf("в журнале текст поиска (email): %q", all)
	}
}

func recsEmail(t *testing.T) string {
	t.Helper()
	return organizersPortfolio(t).Customers[0].Row.Email
}

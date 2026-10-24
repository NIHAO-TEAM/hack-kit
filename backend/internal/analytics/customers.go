package analytics

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/httpx"
)

const (
	defaultLimit = 50
	maxLimit     = 200
)

// ListCustomers — GET /api/v1/customers.
func (a *AnalyticsAPI) ListCustomers(w responseWriter, r *request, p gen.ListCustomersParams) {
	pf := a.portfolio()
	if pf == nil {
		httpx.WriteProblem(w, r, 503, "расчёт ещё не готов")
		return
	}
	limit, offset := defaultLimit, 0
	if p.Limit != nil {
		limit = *p.Limit
	}
	if p.Offset != nil {
		offset = *p.Offset
	}
	if limit < 1 || limit > maxLimit || offset < 0 {
		httpx.WriteProblem(w, r, 400, "limit должен быть 1..200, offset — не меньше 0")
		return
	}
	if p.Level != nil && !validLevel(*p.Level) {
		httpx.WriteProblem(w, r, 400, "неизвестный уровень")
		return
	}
	sortBy, desc := gen.ListCustomersParamsSort("expectedLoss"), true
	if p.Sort != nil {
		sortBy = *p.Sort
	}
	if p.Order != nil {
		desc = *p.Order != gen.Asc
	}
	cmpFn, ok := comparator(sortBy)
	if !ok {
		httpx.WriteProblem(w, r, 400, "неизвестное поле сортировки")
		return
	}

	pinUnscored := sortBy == "score" || sortBy == "expectedLoss"

	rows := make([]gen.CustomerRow, 0, len(pf.Customers))
	q := ""
	if p.Q != nil {
		q = strings.ToLower(strings.TrimSpace(*p.Q))
	}
	for _, c := range pf.Customers {
		if p.Level != nil && c.Row.Level != *p.Level {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(c.Row.Email), q) &&
			!strings.Contains(strconv.FormatInt(c.Row.Id, 10), q) {
			continue
		}
		rows = append(rows, c.Row)
	}
	slices.SortStableFunc(rows, func(x, y gen.CustomerRow) int {
		if pinUnscored { // только по скору и потере: «нет оценки» и «потерян» внизу
			xs, ys := isScored(x), isScored(y)
			switch {
			case xs != ys:
				if xs {
					return -1
				}
				return 1
			case !xs:
				return cmp.Compare(x.Id, y.Id)
			}
		}
		c := cmpFn(x, y)
		if desc {
			c = -c
		}
		if c != 0 {
			return c
		}
		return cmp.Compare(x.Id, y.Id)
	})

	total := len(rows)
	rows = rows[min(offset, total):]
	rows = rows[:min(limit, len(rows))]
	writeJSON(w, r, gen.CustomerPage{Items: rows, Total: total, Version: pf.Version})
}

// GetCustomer — GET /api/v1/customers/{id}.
func (a *AnalyticsAPI) GetCustomer(w responseWriter, r *request, id gen.CustomerId) {
	pf := a.portfolio()
	if pf == nil {
		httpx.WriteProblem(w, r, 503, "расчёт ещё не готов")
		return
	}
	i, found := slices.BinarySearchFunc(pf.Customers, id, func(c Customer, id int64) int {
		return cmp.Compare(c.Row.Id, id)
	})
	if !found {
		httpx.WriteProblem(w, r, 404, "клиент не найден")
		return
	}
	c := pf.Customers[i]
	d := gen.CustomerDetail{
		Version:      pf.Version,
		Customer:     c.Row,
		Account:      &c.Account,
		Services:     orEmpty(c.Services),
		ServiceRisks: &c.Risks,
		Features:     orEmpty(c.Features),
		Actions:      orEmpty(c.Actions),
		Explanation:  c.Row.TopReason,
	}
	writeJSON(w, r, d)
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func isScored(r gen.CustomerRow) bool {
	return r.Level != gen.Unscored && r.Level != gen.Lost
}

func validLevel(l gen.RiskLevel) bool {
	switch l {
	case gen.High, gen.Medium, gen.Low, gen.Unscored, gen.Lost:
		return true
	}
	return false
}

func comparator(s gen.ListCustomersParamsSort) (func(x, y gen.CustomerRow) int, bool) {
	switch s {
	case "id":
		return func(x, y gen.CustomerRow) int { return cmp.Compare(x.Id, y.Id) }, true
	case "email":
		return func(x, y gen.CustomerRow) int { return cmp.Compare(strings.ToLower(x.Email), strings.ToLower(y.Email)) }, true
	case "balance":
		return func(x, y gen.CustomerRow) int { return cmp.Compare(x.Balance, y.Balance) }, true
	case "totalReplenished":
		return func(x, y gen.CustomerRow) int { return cmp.Compare(x.TotalReplenished, y.TotalReplenished) }, true
	case "score":
		return func(x, y gen.CustomerRow) int { return cmp.Compare(deref(x.Score), deref(y.Score)) }, true
	case "expectedLoss":
		return func(x, y gen.CustomerRow) int { return cmp.Compare(x.ExpectedLossCents, y.ExpectedLossCents) }, true
	}
	return nil, false
}

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

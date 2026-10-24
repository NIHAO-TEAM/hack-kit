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
	f := customerFilter{Level: p.Level, Q: p.Q, Sort: p.Sort, Order: p.Order, Billing: p.Billing, Task: p.Task}
	var err error
	if f.Tasks, err = a.openTasks(r.Context()); err != nil && p.Task != nil {
		httpx.WriteProblem(w, r, 500, "задачи недоступны")
		return
	}
	if a.login != nil {
		f.Me = a.login(r.Context())
	}
	rows, bad := selectRows(pf, f)
	if bad != "" {
		httpx.WriteProblem(w, r, 400, bad)
		return
	}

	total := len(rows)
	rows = rows[min(offset, total):]
	rows = rows[:min(limit, len(rows))]
	writeJSON(w, r, gen.CustomerPage{Items: rows, Total: total, Version: pf.Version})
}

// customerFilter — общие параметры списка и выгрузки: фильтры и сортировка без страниц.
type customerFilter struct {
	Level   *gen.RiskLevel
	Q       *string
	Sort    *gen.ListCustomersParamsSort
	Order   *gen.ListCustomersParamsOrder
	Billing *gen.ListCustomersParamsBilling
	// Task, Tasks и Me — фильтр по задачам удержания: открытые задачи по ID клиента и логин
	// текущего пользователя для «mine». Выгрузка их не задаёт, строки идут без task.
	Task  *gen.ListCustomersParamsTask
	Tasks map[int64]TaskRef
	Me    string
}

// selectRows отбирает и сортирует строки портфеля; при неверном параметре вернёт текст для 400.
func selectRows(pf *Portfolio, f customerFilter) (rows []gen.CustomerRow, bad string) {
	if f.Level != nil && !validLevel(*f.Level) {
		return nil, "неизвестный уровень"
	}
	sortBy, desc := gen.ListCustomersParamsSort("expectedLoss"), true
	if f.Sort != nil {
		sortBy = *f.Sort
	}
	if f.Order != nil {
		desc = *f.Order != gen.Asc
	}
	if f.Billing != nil && *f.Billing != gen.Exclude && *f.Billing != gen.Only {
		return nil, "billing: exclude или only"
	}
	cmpFn, ok := comparator(sortBy)
	if !ok {
		return nil, "неизвестное поле сортировки"
	}

	if f.Task != nil && !validTaskFilter(*f.Task) {
		return nil, "task: none, open или mine"
	}

	pinUnscored := sortBy == "score" || sortBy == "expectedLoss"

	rows = make([]gen.CustomerRow, 0, len(pf.Customers))
	q := ""
	if f.Q != nil {
		q = strings.ToLower(strings.TrimSpace(*f.Q))
	}
	for _, c := range pf.Customers {
		if f.Level != nil && c.Row.Level != *f.Level {
			continue
		}
		if f.Billing != nil && isBilling(c.Row) != (*f.Billing == gen.Only) {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(c.Row.Email), q) &&
			!strings.Contains(strconv.FormatInt(c.Row.Id, 10), q) {
			continue
		}
		row := c.Row
		row.Task = briefFor(f.Tasks, row)
		if f.Task != nil {
			switch *f.Task {
			case "none":
				if row.Task != nil {
					continue
				}
			case "open":
				if row.Task == nil {
					continue
				}
			case "mine":
				if row.Task == nil || row.Task.Assignee != f.Me {
					continue
				}
			}
		}
		rows = append(rows, row)
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

	return rows, ""
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
	row := c.Row
	if tasks, err := a.openTasks(r.Context()); err == nil {
		row.Task = briefFor(tasks, row)
	}
	d := gen.CustomerDetail{
		Version:      pf.Version,
		Customer:     row,
		Account:      &c.Account,
		Services:     orEmpty(c.Services),
		ServiceRisks: &c.Risks,
		Features:     orEmpty(c.Features),
		Actions:      orEmpty(c.Actions),
		ActionCombo:  c.ActionCombo,
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

func isBilling(r gen.CustomerRow) bool { return r.Billing != nil && *r.Billing }

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

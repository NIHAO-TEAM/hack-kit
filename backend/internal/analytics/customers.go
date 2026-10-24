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
	after := -1
	if p.Cursor != nil && *p.Cursor != "" {
		if offset != 0 {
			httpx.WriteProblem(w, r, 400, "cursor и offset вместе не задаются")
			return
		}
		c, err := decodeCursor(*p.Cursor)
		if err != nil {
			httpx.WriteProblem(w, r, 400, "cursor не распознан")
			return
		}
		if c.Version != pf.Version {
			httpx.WriteProblem(w, r, 409, "курсор устарел: данные пересчитаны, начните с первой страницы")
			return
		}
		if !c.matches(p.Sort, p.Order) {
			httpx.WriteProblem(w, r, 400, "cursor выдан для другой сортировки")
			return
		}
		after = c.Pos
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
	res, bad := selectPage(pf, f, window{Offset: offset, Limit: limit, After: after})
	if bad != "" {
		httpx.WriteProblem(w, r, 400, bad)
		return
	}
	page := gen.CustomerPage{Items: res.Rows, Total: res.Total, Version: pf.Version}
	if res.More {
		cur := encodeCursor(cursor{Version: pf.Version, Sort: res.Sort, Desc: res.Desc, Pos: res.Last})
		page.NextCursor = &cur
	}
	writeJSON(w, r, page)
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

// selector — разобранные параметры списка: порядок из модели чтения и проверка строки на фильтры.
type selector struct {
	pf    *Portfolio
	f     customerFilter
	order []int32
	sort  gen.ListCustomersParamsSort
	desc  bool
	q     string
}

// newSelector проверяет параметры; при неверном вернёт текст для 400.
func newSelector(pf *Portfolio, f customerFilter) (*selector, string) {
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
	order, ok := pf.index().order(sortBy, desc)
	if !ok {
		return nil, "неизвестное поле сортировки"
	}
	if f.Task != nil && !validTaskFilter(*f.Task) {
		return nil, "task: none, open или mine"
	}
	s := &selector{pf: pf, f: f, order: order, sort: sortBy, desc: desc}
	if f.Q != nil {
		s.q = strings.ToLower(strings.TrimSpace(*f.Q))
	}
	return s, ""
}

// match — проходит ли клиент фильтры; строку не копирует.
func (s *selector) match(row *gen.CustomerRow) bool {
	f := s.f
	if f.Level != nil && row.Level != *f.Level {
		return false
	}
	if f.Billing != nil && isBilling(*row) != (*f.Billing == gen.Only) {
		return false
	}
	if s.q != "" && !strings.Contains(strings.ToLower(row.Email), s.q) &&
		!strings.Contains(strconv.FormatInt(row.Id, 10), s.q) {
		return false
	}
	if f.Task != nil {
		task := briefFor(f.Tasks, *row)
		switch *f.Task {
		case "none":
			return task == nil
		case "open":
			return task != nil
		case "mine":
			return task != nil && task.Assignee == f.Me
		}
	}
	return true
}

func (s *selector) row(pos int32) gen.CustomerRow {
	row := s.pf.Customers[pos].Row
	row.Task = briefFor(s.f.Tasks, row)
	return row
}

// selectRows отбирает и сортирует все строки портфеля (для выгрузки); при неверном параметре вернёт текст для 400.
func selectRows(pf *Portfolio, f customerFilter) (rows []gen.CustomerRow, bad string) {
	s, bad := newSelector(pf, f)
	if bad != "" {
		return nil, bad
	}
	rows = make([]gen.CustomerRow, 0, len(s.order))
	for _, pos := range s.order {
		if s.match(&s.pf.Customers[pos].Row) {
			rows = append(rows, s.row(pos))
		}
	}
	return rows, ""
}

// window — какую часть выборки отдать: либо Offset строк пропустить, либо идти после позиции After
// в порядке модели чтения (−1 — с начала).
type window struct{ Offset, Limit, After int }

type pageResult struct {
	Rows  []gen.CustomerRow
	Total int
	// Last — позиция последней отданной строки в порядке модели чтения; More — есть ли строки дальше.
	Last int
	More bool
	Sort gen.ListCustomersParamsSort
	Desc bool
}

// selectPage идёт по готовому порядку: считает total по всей выборке, а строки копирует только страницы.
func selectPage(pf *Portfolio, f customerFilter, win window) (pageResult, string) {
	s, bad := newSelector(pf, f)
	if bad != "" {
		return pageResult{}, bad
	}
	res := pageResult{Rows: make([]gen.CustomerRow, 0, win.Limit), Sort: s.sort, Desc: s.desc, Last: -1}
	skipped := 0
	for k, pos := range s.order {
		if !s.match(&pf.Customers[pos].Row) {
			continue
		}
		res.Total++
		switch {
		case k <= win.After:
		case skipped < win.Offset:
			skipped++
		case len(res.Rows) < win.Limit:
			res.Rows = append(res.Rows, s.row(pos))
			res.Last = k
		default:
			res.More = true
		}
	}
	return res, ""
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

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

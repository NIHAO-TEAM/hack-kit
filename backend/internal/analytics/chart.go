package analytics

import (
	"math"
	"time"

	"example.com/app/internal/platform/api/gen"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Горизонт графика в неделях: назад — факт удалений, вперёд — прогноз непродлений.
const (
	chartWeeksBack    = 8
	chartWeeksForward = 8
)

// chartService — то, что нужно графику от услуги: срок, статус и вероятность непродления.
type chartService struct {
	ExpiresAt time.Time
	Deleted   bool
	P         float64
}

// weekStart — понедельник недели, в которую попадает t (UTC).
func weekStart(t time.Time) time.Time {
	t = t.UTC()
	d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	back := (int(d.Weekday()) + 6) % 7
	return d.AddDate(0, 0, -back)
}

// buildChurnChart — чистая функция графика. Факт: удалённые услуги по неделе даты удаления
// (срок + окно спасения [ПРЕДПОЛОЖЕНО]), только до текущей недели включительно. Прогноз:
// Σ P живых услуг, срок которых наступает в неделю, от текущей на chartWeeksForward вперёд.
// Текущая неделя несёт оба значения, чтобы линии на графике стыковались.
func buildChurnChart(svcs []chartService, now time.Time) []gen.ChurnPoint {
	cur := weekStart(now)
	first := cur.AddDate(0, 0, -7*chartWeeksBack)
	n := chartWeeksBack + chartWeeksForward + 1
	actual := make([]int, n)
	forecast := make([]float64, n)
	idx := func(t time.Time) int {
		return int(weekStart(t).Sub(first).Hours() / (24 * 7))
	}
	curIdx := chartWeeksBack
	for _, s := range svcs {
		if s.Deleted {
			if i := idx(s.ExpiresAt.Add(Window)); i >= 0 && i <= curIdx {
				actual[i]++
			}
			continue
		}
		if s.ExpiresAt.Before(now) {
			continue
		}
		if i := idx(s.ExpiresAt); i >= curIdx && i < n {
			forecast[i] += s.P
		}
	}
	pts := make([]gen.ChurnPoint, n)
	for i := range pts {
		pts[i].WeekStart = openapi_types.Date{Time: first.AddDate(0, 0, 7*i)}
		if i <= curIdx {
			a := actual[i]
			pts[i].Actual = &a
		}
		if i >= curIdx {
			f := int(math.Round(forecast[i]))
			pts[i].Forecast = &f
		}
	}
	return pts
}

// GetChurnChart — GET /api/v1/analytics/churn-chart.
func (a *AnalyticsAPI) GetChurnChart(w responseWriter, r *request) {
	pf := a.portfolio()
	if pf == nil {
		notReady(w, r)
		return
	}
	now := a.clk.Now()
	if o := a.latest(); o != nil && !o.AsOf.IsZero() {
		now = o.AsOf
	}
	var svcs []chartService
	for _, c := range pf.Customers {
		p := make(map[int64]float64, len(c.Risks))
		for _, rk := range c.Risks {
			p[rk.ServiceId] = rk.Probability
		}
		for _, s := range c.Services {
			svcs = append(svcs, chartService{ExpiresAt: s.ExpiresAt, Deleted: s.Status == gen.Deleted, P: p[s.Id]})
		}
	}
	writeJSON(w, r, gen.ChurnChart{Points: buildChurnChart(svcs, now)})
}

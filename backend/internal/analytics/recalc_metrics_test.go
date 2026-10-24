package analytics_test

import (
	"strings"
	"testing"
	"time"

	"example.com/app/internal/analytics"
)

func TestRecalcMetrics(t *testing.T) {
	m := analytics.NewRecalcMetrics(func(int) int { return 4 })
	m.Cancelled = func() int64 { return 3 }
	m.ObserveBatch(256, 2*time.Millisecond)
	m.ObserveBatch(256, 3*time.Millisecond)

	api := newAPI(fixture()).WithCollectors(m)
	body := get(api.GetMetrics, "/metrics").Body.String()
	for _, want := range []string{
		"churn_recalc_workers 4",
		"churn_recalc_cancelled_total 3",
		"churn_recalc_batch_seconds_count 2",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("в /metrics нет %q", want)
		}
	}
}

// 2.4: сколько клиентов пересчитано и сколько длились частичные и полные пересчёты — отдельно.
func TestRecalcCustomersAndModeMetrics(t *testing.T) {
	m := analytics.NewRecalcMetrics(func(int) int { return 4 })
	m.ObserveRecalc(true, 3*time.Millisecond)
	m.ObserveRecalc(true, 5*time.Millisecond)
	m.ObserveRecalc(false, 800*time.Millisecond)
	o := fixture()
	o.RecalcCustomers, o.Partial = 1, true

	api := newAPI(o).WithCollectors(m)
	body := get(api.GetMetrics, "/metrics").Body.String()
	for _, want := range []string{
		"churn_recalc_customers 1\n",
		`churn_recalc_mode_seconds_count{mode="partial"} 2`,
		`churn_recalc_mode_seconds_count{mode="full"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("в /metrics нет %q", want)
		}
	}
}

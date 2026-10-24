package analytics_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil/promlint"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"

	"example.com/app/internal/analytics"
	"example.com/app/internal/platform/clock"
)

var asOf = time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC)

// fixture — сводка демо-сценария: уровни 3/2/1/0/4 и суммы из specs/metrics.md.
func fixture() *analytics.Overview {
	return &analytics.Overview{
		Version: 7, AsOf: asOf, Duration: 12 * time.Millisecond,
		Levels:            map[string]int{"high": 3, "medium": 2, "low": 1, "unscored": 0, "lost": 4},
		ExpectedLossCents: 359907, AtRiskCents: 690046, LostCents: 736834,
		QualityCounts: map[string]int{"stale": 2},
	}
}

func newAPI(o *analytics.Overview) *analytics.AnalyticsAPI {
	clk := clock.NewFake(asOf.Add(time.Hour))
	return analytics.New(nil, nil, clk).WithOverview(func() *analytics.Overview { return o })
}

func get(h http.HandlerFunc, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestSummary(t *testing.T) {
	rec := get(newAPI(fixture()).GetSummary, "/api/v1/analytics/summary")
	if rec.Code != 200 {
		t.Fatalf("статус %d: %s", rec.Code, rec.Body)
	}
	var got struct {
		Levels            map[string]int `json:"levels"`
		ExpectedLossCents int64          `json:"expectedLossCents"`
		AtRiskCents       int64          `json:"atRiskCents"`
		LostCents         int64          `json:"lostCents"`
		Version           int64          `json:"version"`
		AsOf              time.Time      `json:"asOf"`
		QualityCounts     map[string]int `json:"qualityCounts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"high": 3, "medium": 2, "low": 1, "unscored": 0, "lost": 4}
	for k, v := range want {
		if got.Levels[k] != v {
			t.Errorf("уровень %s = %d, ждали %d", k, got.Levels[k], v)
		}
	}
	if got.ExpectedLossCents != 359907 || got.AtRiskCents != 690046 || got.LostCents != 736834 || got.Version != 7 {
		t.Errorf("суммы: %+v", got)
	}
	if !got.AsOf.Equal(asOf) || got.QualityCounts["stale"] != 2 {
		t.Errorf("asOf/качество: %+v", got)
	}
}

func TestSummaryBeforeFirstRecalc(t *testing.T) {
	rec := get(newAPI(nil).GetSummary, "/api/v1/analytics/summary")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("статус %d, ждали 503", rec.Code)
	}
}

func parse(t *testing.T, body string) map[string]*dto.MetricFamily {
	t.Helper()
	parser := expfmt.NewTextParser(model.UTF8Validation)
	fams, err := parser.TextToMetricFamilies(strings.NewReader(body))
	if err != nil {
		t.Fatalf("разбор формата Prometheus: %v\n%s", err, body)
	}
	return fams
}

func TestMetricsMatchSummary(t *testing.T) {
	api := newAPI(fixture())
	rec := get(api.GetMetrics, "/metrics")
	if rec.Code != 200 {
		t.Fatalf("статус %d", rec.Code)
	}
	fams := parse(t, rec.Body.String())

	summary := get(api.GetSummary, "/api/v1/analytics/summary")
	var s struct {
		Levels            map[string]float64 `json:"levels"`
		ExpectedLossCents float64            `json:"expectedLossCents"`
		AtRiskCents       float64            `json:"atRiskCents"`
		LostCents         float64            `json:"lostCents"`
		Version           float64            `json:"version"`
	}
	if err := json.Unmarshal(summary.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}

	levels := map[string]float64{}
	var sum float64
	for _, m := range fams["churn_customers"].GetMetric() {
		levels[m.GetLabel()[0].GetValue()] = m.GetGauge().GetValue()
		sum += m.GetGauge().GetValue()
	}
	if len(levels) != 5 {
		t.Errorf("уровней в метриках %d, ждали 5", len(levels))
	}
	for k, v := range s.Levels {
		if levels[k] != v {
			t.Errorf("churn_customers{level=%q} = %v, в сводке %v", k, levels[k], v)
		}
	}
	if sum != 10 {
		t.Errorf("Σ churn_customers = %v, ждали 10", sum)
	}
	one := func(name string) float64 { return fams[name].GetMetric()[0].GetGauge().GetValue() }
	for name, want := range map[string]float64{
		"churn_expected_loss_cents":     s.ExpectedLossCents,
		"churn_at_risk_cents":           s.AtRiskCents,
		"churn_lost_cents":              s.LostCents,
		"churn_model_version":           s.Version,
		"churn_snapshot_age_seconds":    3600,
		"churn_recalc_duration_seconds": 0.012,
	} {
		if got := one(name); got != want {
			t.Errorf("%s = %v, ждали %v", name, got, want)
		}
	}
	if got := fams["churn_data_quality_issues"].GetMetric()[0].GetGauge().GetValue(); got != 2 {
		t.Errorf("churn_data_quality_issues = %v", got)
	}
	for _, name := range []string{"go_goroutines", "process_start_time_seconds"} {
		if _, ok := fams[name]; !ok {
			t.Logf("нет стандартной метрики %s (на этой платформе)", name)
		}
	}
}

func TestMetricsLint(t *testing.T) {
	rec := get(newAPI(fixture()).GetMetrics, "/metrics")
	problems, err := promlint.New(strings.NewReader(rec.Body.String())).Lint()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		if strings.HasPrefix(p.Metric, "churn_") {
			t.Errorf("promlint: %s: %s", p.Metric, p.Text)
		}
	}
}

func TestMetricsAliasSameAsMetrics(t *testing.T) {
	api := newAPI(fixture())
	a, b := get(api.GetMetrics, "/metrics"), get(api.GetMetricsAlias, "/metics")
	if a.Code != 200 || b.Code != 200 {
		t.Fatalf("статусы %d %d", a.Code, b.Code)
	}
	only := func(r *httptest.ResponseRecorder) string {
		body, _ := io.ReadAll(r.Body)
		var keep []string
		for _, l := range strings.Split(string(body), "\n") {
			if strings.HasPrefix(l, "churn_") || strings.HasPrefix(l, "# HELP churn_") {
				keep = append(keep, l)
			}
		}
		return strings.Join(keep, "\n")
	}
	wantText, gotText := only(a), only(b)
	if wantText != gotText || wantText == "" {
		t.Errorf("/metics отличается от /metrics")
	}
}

func TestMetricsBeforeFirstRecalc(t *testing.T) {
	rec := get(newAPI(nil).GetMetrics, "/metrics")
	if rec.Code != 200 || strings.Contains(rec.Body.String(), "churn_customers") {
		t.Fatalf("до первого расчёта ждали 200 без churn_*, получили %d", rec.Code)
	}
}

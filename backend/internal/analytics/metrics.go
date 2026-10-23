package analytics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"example.com/app/internal/platform/clock"
)

var (
	descCustomers = prometheus.NewDesc("churn_customers",
		"Число клиентов по уровню риска (штуки); level: high, medium, low, unscored, lost.", []string{"level"}, nil)
	descExpectedLoss = prometheus.NewDesc("churn_expected_loss_cents",
		"Ожидаемая потеря по всем клиентам (центы евро).", nil, nil)
	descAtRisk = prometheus.NewDesc("churn_at_risk_cents",
		"Под угрозой: платежи на горизонт клиентов высокого и среднего риска (центы евро).", nil, nil)
	descLost = prometheus.NewDesc("churn_lost_cents",
		"Уже потеряно: платежи на горизонт потерянных клиентов (центы евро).", nil, nil)
	descQuality = prometheus.NewDesc("churn_data_quality_issues",
		"Число случаев по проверке качества данных (штуки); check — имя проверки.", []string{"check"}, nil)
	descRecalc = prometheus.NewDesc("churn_recalc_duration_seconds",
		"Длительность последнего пересчёта (секунды).", nil, nil)
	descAge = prometheus.NewDesc("churn_snapshot_age_seconds",
		"Возраст данных снимка: сейчас минус момент снимка (секунды).", nil, nil)
	descVersion = prometheus.NewDesc("churn_model_version",
		"Номер версии последнего пересчёта (растёт на 1 с каждым пересчётом).", nil, nil)
)

// overviewCollector читает последний результат при каждом сборе, поэтому метрики и сводка
// всегда берутся из одного результата.
type overviewCollector struct {
	overview OverviewFunc
	clk      clock.Clock
}

func (overviewCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{descCustomers, descExpectedLoss, descAtRisk, descLost,
		descQuality, descRecalc, descAge, descVersion} {
		ch <- d
	}
}

func (c overviewCollector) Collect(ch chan<- prometheus.Metric) {
	if c.overview == nil {
		return
	}
	o := c.overview()
	if o == nil {
		return
	}
	gauge := func(d *prometheus.Desc, v float64, labels ...string) {
		ch <- prometheus.MustNewConstMetric(d, prometheus.GaugeValue, v, labels...)
	}
	for _, level := range levelNames {
		gauge(descCustomers, float64(o.Levels[level]), level)
	}
	gauge(descExpectedLoss, float64(o.ExpectedLossCents))
	gauge(descAtRisk, float64(o.AtRiskCents))
	gauge(descLost, float64(o.LostCents))
	for check, n := range o.QualityCounts {
		gauge(descQuality, float64(n), check)
	}
	gauge(descRecalc, o.Duration.Seconds())
	if !o.AsOf.IsZero() {
		gauge(descAge, c.clk.Now().Sub(o.AsOf).Seconds())
	}
	gauge(descVersion, float64(o.Version))
}

// newRegistry собирает свой реестр: метрики модели и стандартные метрики процесса.
func newRegistry(overview OverviewFunc, clk clock.Clock, extra ...prometheus.Collector) *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		overviewCollector{overview: overview, clk: clk},
	)
	reg.MustRegister(extra...)
	return reg
}

// GetMetrics — GET /metrics.
func (a *AnalyticsAPI) GetMetrics(w responseWriter, r *request) {
	promhttp.HandlerFor(a.registry(), promhttp.HandlerOpts{}).ServeHTTP(w, r)
}

// GetMetricsAlias — GET /metics, алиас опечатки в кейсе: тот же обработчик.
func (a *AnalyticsAPI) GetMetricsAlias(w responseWriter, r *request) { a.GetMetrics(w, r) }

func (a *AnalyticsAPI) registry() *prometheus.Registry {
	a.regOnce.Do(func() { a.reg = newRegistry(a.overview, a.clk, a.extra...) })
	return a.reg
}

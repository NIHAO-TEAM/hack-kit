package analytics

import (
	"math"
	"time"

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
	descStage = prometheus.NewDesc("churn_recalc_stage_seconds",
		"Длительность этапов последнего пересчёта (секунды); stage: load, convert, score.", []string{"stage"}, nil)
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
		descQuality, descRecalc, descStage, descAge, descVersion} {
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
	gauge(descStage, o.Stages.Load.Seconds(), "load")
	gauge(descStage, o.Stages.Convert.Seconds(), "convert")
	gauge(descStage, o.Stages.Score.Seconds(), "score")
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

// RecalcMetrics — метрики конвейера пересчёта: число воркеров, отменённые прогоны, длительность пачек.
// Модуль не знает ядро модели: число воркеров и счётчик отмен ему передаёт сборка сервера.
type RecalcMetrics struct {
	// Workers даёт число воркеров для снимка из n клиентов; nil — не показывать.
	Workers   func(n int) int
	Cancelled func() int64

	batches prometheus.Histogram
}

// NewRecalcMetrics создаёт метрики; workers считает воркеров по числу клиентов (risk.Workers).
func NewRecalcMetrics(workers func(n int) int) *RecalcMetrics {
	return &RecalcMetrics{
		Workers: workers,
		batches: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "churn_recalc_batch_seconds",
			Help:    "Длительность расчёта одной пачки клиентов воркером (секунды).",
			Buckets: prometheus.ExponentialBuckets(0.0005, 2, 12),
		}),
	}
}

// ObserveBatch записывает длительность пачки; безопасен из нескольких воркеров.
func (m *RecalcMetrics) ObserveBatch(_ int, took time.Duration) { m.batches.Observe(took.Seconds()) }

var (
	descWorkers = prometheus.NewDesc("churn_recalc_workers",
		"Число воркеров пересчёта (по числу ядер, runtime.GOMAXPROCS).", nil, nil)
	descCancelled = prometheus.NewDesc("churn_recalc_cancelled_total",
		"Сколько пересчётов отменено новым событием снимка или настроек.", nil, nil)
)

func (m *RecalcMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- descWorkers
	ch <- descCancelled
	m.batches.Describe(ch)
}

func (m *RecalcMetrics) Collect(ch chan<- prometheus.Metric) {
	if m.Workers != nil {
		ch <- prometheus.MustNewConstMetric(descWorkers, prometheus.GaugeValue, float64(m.Workers(math.MaxInt32)))
	}
	if m.Cancelled != nil {
		ch <- prometheus.MustNewConstMetric(descCancelled, prometheus.CounterValue, float64(m.Cancelled()))
	}
	m.batches.Collect(ch)
}

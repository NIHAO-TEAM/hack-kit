package source

import (
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Этапы загрузки снимка в метрике churn_snapshot_ingest_stage_seconds{stage}.
const (
	stageRead             = "read"
	stageParse            = "parse"
	stageValidate         = "validate"
	stageLock             = "lock"
	stageTruncate         = "truncate"
	stageCopyAccounts     = "copy_accounts"
	stageCopyServices     = "copy_services"
	stageCopyTransactions = "copy_transactions"
	stageSequences        = "sequences"
	stageCommit           = "commit"
)

var ingestStage = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "churn_snapshot_ingest_stage_seconds",
	Help:    "Длительность этапа загрузки снимка (секунды); stage: read, parse, validate, lock, truncate, copy_*, sequences, commit.",
	Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2, 5, 10, 20, 30, 60},
}, []string{"stage"})

// IngestCollectors — метрики этапов загрузки для /metrics.
func IngestCollectors() []prometheus.Collector { return []prometheus.Collector{ingestStage} }

// stages копит длительности этапов одной загрузки и пишет их в лог одной строкой.
type stages struct {
	names []string
	durs  []time.Duration
}

// run выполняет f и записывает длительность этапа name (в том числе при ошибке).
func (s *stages) run(name string, f func() error) error {
	start := time.Now()
	err := f()
	d := time.Since(start)
	ingestStage.WithLabelValues(name).Observe(d.Seconds())
	s.names = append(s.names, name)
	s.durs = append(s.durs, d)
	return err
}

// log печатает все этапы: «source: этапы загрузки: parse=1.2s validate=0.3s …».
func (s *stages) log() {
	line := ""
	for i, n := range s.names {
		line += " " + n + "=" + s.durs[i].Round(time.Millisecond).String()
	}
	log.Printf("source: этапы загрузки:%s", line)
}

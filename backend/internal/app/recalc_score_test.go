package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"example.com/app/internal/analytics"
	"example.com/app/internal/risk"
)

// Боевой путь расчёта идёт через конвейер: метрика пачек наполняется, а отмена обрывает сам расчёт.
func TestScoreSnapshotUsesPipeline(t *testing.T) {
	snap := risk.Snapshot{AsOf: time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC)}
	for i := int64(1); i <= 600; i++ {
		snap.Accounts = append(snap.Accounts, risk.Account{ID: i, Email: "a@example.com"})
	}
	m := analytics.NewRecalcMetrics(risk.Workers)
	if _, err := scoreSnapshot(context.Background(), snap, risk.Defaults(), m); err != nil {
		t.Fatal(err)
	}
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(m)
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var batches uint64
	for _, f := range families {
		if f.GetName() == "churn_recalc_batch_seconds" {
			batches = f.GetMetric()[0].GetHistogram().GetSampleCount()
		}
	}
	if batches != 3 { // 600 клиентов, пачка 256
		t.Errorf("пачек в метрике %d, ждали 3", batches)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := scoreSnapshot(ctx, snap, risk.Defaults(), m); !errors.Is(err, context.Canceled) {
		t.Errorf("отменённый расчёт: ошибка %v, ждали context.Canceled", err)
	}
}

package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"example.com/app/internal/risk"
)

func organizersSnapshot(t *testing.T) risk.Snapshot {
	t.Helper()
	raw, err := os.ReadFile("../../../cases/analizator/data.json")
	if err != nil {
		t.Fatal(err)
	}
	var snap risk.Snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	snap.AsOf = time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC)
	return snap
}

// Конвейер даёт побитово тот же результат, что последовательный расчёт, при любом размере пачки.
func TestPipelineEqualsSequential(t *testing.T) {
	cases := map[string]risk.Snapshot{
		"организаторы":  organizersSnapshot(t),
		"синтетика 10k": syntheticSnapshot(t, 10_000),
	}
	for name, rs := range cases {
		t.Run(name, func(t *testing.T) {
			prev := runtime.GOMAXPROCS(1)
			seq := risk.Score(rs, risk.Defaults(), rs.AsOf)
			runtime.GOMAXPROCS(8)
			defer runtime.GOMAXPROCS(prev)
			for _, batch := range []int{0, 1, 7, 256, 1024} {
				var batches atomic.Int64
				par, err := risk.ScoreContext(context.Background(), rs, risk.Defaults(), rs.AsOf, risk.Options{
					BatchSize: batch,
					OnBatch:   func(int, time.Duration) { batches.Add(1) },
				})
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(seq, par) {
					t.Errorf("пачка %d: результат отличается от последовательного", batch)
				}
				if batches.Load() == 0 {
					t.Errorf("пачка %d: наблюдатель не вызывался", batch)
				}
			}
		})
	}
}

// Отменённый расчёт останавливается между пачками и не отдаёт частичный результат.
func TestScoreContextCancel(t *testing.T) {
	rs := syntheticSnapshot(t, 10_000)
	ctx, cancel := context.WithCancel(context.Background())
	var done atomic.Int64
	res, err := risk.ScoreContext(ctx, rs, risk.Defaults(), rs.AsOf, risk.Options{
		BatchSize: 64,
		OnBatch: func(n int, _ time.Duration) {
			if done.Add(int64(n)) >= 64 {
				cancel()
			}
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ошибка = %v, ждали context.Canceled", err)
	}
	if res.Customers != nil || done.Load() >= 10_000 {
		t.Errorf("расчёт не остановился: посчитано %d, клиентов в результате %d", done.Load(), len(res.Customers))
	}
}

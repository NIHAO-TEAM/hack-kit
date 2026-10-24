package app

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"example.com/app/internal/analytics"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/risk"
)

// Читатели, пришедшие за новой версией одновременно, строят портфель один раз, а не каждый свой.
func TestPortfolioBuiltOncePerVersion(t *testing.T) {
	bus := events.New()
	compute := func(context.Context) (analytics.Computed[risk.Result], error) {
		return analytics.Computed[risk.Result]{DataVersion: 3}, nil
	}
	eng := analytics.NewEngine(compute, bus, clock.NewFake(time.Now()), "t")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	eng.Start(ctx)
	for deadline := time.Now().Add(5 * time.Second); eng.Latest() == nil; {
		if time.Now().After(deadline) {
			t.Fatal("пересчёт не завершился")
		}
		time.Sleep(time.Millisecond)
	}
	var builds atomic.Int64
	r := &Recalc{Engine: eng, snaps: &snapshots{}}
	r.snaps.put(3, risk.Snapshot{})
	r.build = func(risk.Result, risk.Snapshot, int64) *analytics.Portfolio {
		builds.Add(1)
		time.Sleep(50 * time.Millisecond) // сборка идёт дольше, чем приходят читатели
		return &analytics.Portfolio{}
	}
	var wg sync.WaitGroup
	got := make([]*analytics.Portfolio, 20)
	for i := range got {
		wg.Add(1)
		go func() { defer wg.Done(); got[i] = r.Portfolio() }()
	}
	wg.Wait()
	if n := builds.Load(); n != 1 {
		t.Fatalf("сборок портфеля %d, нужна 1", n)
	}
	for i, pf := range got {
		if pf == nil || pf != got[0] {
			t.Fatalf("читатель %d получил другой портфель", i)
		}
	}
}

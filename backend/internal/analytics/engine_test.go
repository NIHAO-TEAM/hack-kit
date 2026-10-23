package analytics_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"example.com/app/internal/analytics"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
)

const (
	topicData     = "source.changed"
	topicSettings = "model.changed"
)

// inputs — «база» теста: версии, которые читает расчёт.
type inputs struct {
	data, settings atomic.Int64
}

func (in *inputs) compute(context.Context) (analytics.Computed[string], error) {
	return analytics.Computed[string]{Result: "ok", DataVersion: in.data.Load(), SettingsVersion: in.settings.Load()}, nil
}

func start(t *testing.T, compute analytics.ComputeFunc[string]) (*analytics.Engine[string], *events.Bus, <-chan events.Event) {
	t.Helper()
	bus := events.New()
	recalculated, cancelSub := bus.Subscribe(analytics.TopicRecalculated)
	t.Cleanup(cancelSub)
	e := analytics.NewEngine(compute, bus, clock.NewFake(time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)), topicData, topicSettings)
	ctx, cancel := context.WithCancel(context.Background())
	done := e.Start(ctx)
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return e, bus, recalculated
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("не дождались: %s", what)
		}
		time.Sleep(time.Millisecond)
	}
}

func TestRecalcAtStartAndOnEvents(t *testing.T) {
	in := &inputs{}
	in.data.Store(1)
	e, bus, recalculated := start(t, in.compute)

	waitFor(t, "пересчёт при старте", func() bool { return e.Latest() != nil })
	first := e.Latest()
	if first.Version != 1 || first.DataVersion != 1 || first.SettingsVersion != 0 {
		t.Fatalf("первый результат: %+v", first)
	}
	if ev := <-recalculated; ev.Payload != int64(1) {
		t.Errorf("recalculated = %v, ждали 1", ev.Payload)
	}

	in.settings.Store(1)
	bus.Publish(topicSettings, int64(1))
	waitFor(t, "пересчёт по model.changed", func() bool { return e.Latest().Version == 2 })
	if got := e.Latest(); got.SettingsVersion != 1 || got.DataVersion != 1 {
		t.Errorf("после настроек: %+v", got)
	}

	in.data.Store(2)
	bus.Publish(topicData, int64(2))
	waitFor(t, "пересчёт по source.changed", func() bool { return e.Latest().Version == 3 })
	if got := e.Latest(); got.DataVersion != 2 || got.SettingsVersion != 1 {
		t.Errorf("после снимка: %+v", got)
	}
}

func TestBurstOfEventsMakesOneExtraRecalc(t *testing.T) {
	in := &inputs{}
	var calls atomic.Int64
	entered := make(chan struct{}, 100)
	gate := make(chan struct{})
	compute := func(ctx context.Context) (analytics.Computed[string], error) {
		n := calls.Add(1)
		if n == 2 {
			entered <- struct{}{}
			<-gate // держим второй расчёт, пока идёт пачка событий
		}
		return in.compute(ctx)
	}
	e, bus, _ := start(t, compute)
	waitFor(t, "пересчёт при старте", func() bool { return e.Latest() != nil })

	in.data.Store(1)
	bus.Publish(topicData, int64(1))
	<-entered
	for i := int64(2); i <= 10; i++ {
		in.data.Store(i)
		bus.Publish(topicData, i)
	}
	close(gate)

	waitFor(t, "последние данные", func() bool { return e.Latest().DataVersion == 10 })
	time.Sleep(20 * time.Millisecond)
	if n := calls.Load() - 1; n > 2 { // минус пересчёт при старте
		t.Errorf("на 10 событий подряд %d пересчётов, ждали не больше 2", n)
	}
	if got := e.Latest(); got.DataVersion != 10 {
		t.Errorf("последний результат: %+v", got)
	}
}

func TestErrorKeepsPreviousResult(t *testing.T) {
	in := &inputs{}
	var fail atomic.Bool
	compute := func(ctx context.Context) (analytics.Computed[string], error) {
		if fail.Load() {
			return analytics.Computed[string]{}, errors.New("база недоступна")
		}
		return in.compute(ctx)
	}
	e, bus, _ := start(t, compute)
	waitFor(t, "пересчёт при старте", func() bool { return e.Latest() != nil })
	before := e.Latest()

	fail.Store(true)
	bus.Publish(topicData, int64(1))
	waitFor(t, "ошибка засчитана", func() bool { return e.Errors() == 1 })
	if e.Latest() != before {
		t.Errorf("результат изменился после ошибки: %+v", e.Latest())
	}

	fail.Store(false)
	in.data.Store(3)
	bus.Publish(topicData, int64(3))
	waitFor(t, "восстановление", func() bool { return e.Latest().DataVersion == 3 })
	if e.Latest().Version != before.Version+1 {
		t.Errorf("версия после ошибки: %d, ждали %d", e.Latest().Version, before.Version+1)
	}
}

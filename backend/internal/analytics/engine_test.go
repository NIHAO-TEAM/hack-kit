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
	bus := events.New()
	e := analytics.NewEngine(compute, bus, clock.NewFake(time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)), topicData, topicSettings)
	ctx, cancel := context.WithCancel(context.Background())
	done := e.Start(ctx)
	t.Cleanup(func() {
		cancel()
		<-done
	})
	waitFor(t, "пересчёт при старте", func() bool { return e.Latest() != nil })

	in.data.Store(1)
	bus.Publish(topicData, int64(1))
	<-entered
	// Пачка взводится синхронно: шина доставляет события воркеру асинхронно, и опоздавшее
	// событие законно запустило бы ещё один пересчёт уже после gate.
	for i := int64(2); i <= 10; i++ {
		in.data.Store(i)
		e.MarkDirty()
	}
	close(gate)

	// Второй расчёт отменён первым событием пачки и ничего не опубликовал: версия 2 — расчёт после пачки.
	waitFor(t, "пересчёт после пачки", func() bool { return e.Latest().Version == 2 })
	// Останов движка вместо Sleep: после него новых расчётов быть не может.
	cancel()
	<-done
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

// Новое событие отменяет идущий пересчёт: он ничего не публикует, следующий считает свежие данные.
func TestNewEventCancelsRunningRecalc(t *testing.T) {
	in := &inputs{}
	var calls atomic.Int64
	entered := make(chan struct{}, 10)
	compute := func(ctx context.Context) (analytics.Computed[string], error) {
		if calls.Add(1) == 2 {
			entered <- struct{}{}
			<-ctx.Done() // долгий расчёт, который ждёт отмены
			return analytics.Computed[string]{}, ctx.Err()
		}
		return in.compute(ctx)
	}
	e, bus, recalculated := start(t, compute)
	waitFor(t, "пересчёт при старте", func() bool { return e.Latest() != nil })
	<-recalculated

	in.data.Store(1)
	bus.Publish(topicData, int64(1))
	<-entered
	in.data.Store(2)
	bus.Publish(topicData, int64(2)) // второй снимок, пока считается первый

	waitFor(t, "пересчёт по второму снимку", func() bool { return e.Latest().DataVersion == 2 })
	if e.Cancelled() != 1 {
		t.Errorf("отменённых пересчётов %d, ждали 1", e.Cancelled())
	}
	if e.Errors() != 0 {
		t.Errorf("отмена засчитана как ошибка: %d", e.Errors())
	}
	if got := e.Latest(); got.Version != 2 {
		t.Errorf("версия %d, ждали 2: отменённый прогон не должен публиковаться", got.Version)
	}
}

// Поток событий не отменяет пересчёты бесконечно: прогон после отменённого доходит до конца.
func TestEventStreamStillCompletes(t *testing.T) {
	in := &inputs{}
	compute := func(ctx context.Context) (analytics.Computed[string], error) {
		select { // расчёт 5 мс, отменяется
		case <-time.After(5 * time.Millisecond):
		case <-ctx.Done():
			return analytics.Computed[string]{}, ctx.Err()
		}
		return in.compute(ctx)
	}
	e, _, _ := start(t, compute)
	waitFor(t, "пересчёт при старте", func() bool { return e.Latest() != nil })

	stop := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for i := int64(1); i <= 2500; i++ {
			select {
			case <-stop:
				return
			default:
			}
			in.data.Store(i)
			e.MarkDirty()
			time.Sleep(100 * time.Microsecond)
		}
	}()
	defer func() { close(stop); <-finished }()

	// Пока идёт поток, результаты публикуются (не только после его конца).
	waitFor(t, "публикации во время потока", func() bool { return e.Latest().Version >= 3 })
}

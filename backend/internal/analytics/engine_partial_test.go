package analytics_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"example.com/app/internal/analytics"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
)

const topicAccounts = "accounts.changed"

// partialRig — воркер с частичным пересчётом; журнал показывает, какой путь и с какими клиентами выбран.
type partialRig struct {
	e   *analytics.Engine[string]
	bus *events.Bus
	clk *clock.Fake

	mu      sync.Mutex
	log     []string
	gate    chan struct{} // если не nil, частичный пересчёт ждёт ctx или закрытия канала
	refuse  bool
	started chan struct{}
}

func (r *partialRig) calls() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.log)
}

func (r *partialRig) add(s string) {
	r.mu.Lock()
	r.log = append(r.log, s)
	r.mu.Unlock()
}

func newPartialRig(t *testing.T, fullEvery time.Duration, maxShare float64, setup ...func(*analytics.Engine[string])) *partialRig {
	t.Helper()
	r := &partialRig{bus: events.New(), clk: clock.NewFake(time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)), started: make(chan struct{}, 8)}
	r.e = analytics.NewEngine(func(context.Context) (analytics.Computed[string], error) {
		r.add("full")
		return analytics.Computed[string]{Result: "full", Customers: 100}, nil
	}, r.bus, r.clk, topicAccounts, topicData).WithPartial(analytics.Partial[string]{
		Touched: func(topic string, payload any) ([]int64, int64) {
			if ids, ok := payload.([]int64); ok && topic == topicAccounts {
				return ids, 5
			}
			return nil, 0
		},
		Compute: func(ctx context.Context, prev *analytics.Calculation[string], ids []int64, dataVersion, version int64) (analytics.Computed[string], error) {
			slices.Sort(ids)
			r.mu.Lock()
			gate, refuse := r.gate, r.refuse
			r.mu.Unlock()
			r.started <- struct{}{}
			if gate != nil {
				select {
				case <-ctx.Done():
					r.add("cancelled")
					return analytics.Computed[string]{}, ctx.Err()
				case <-gate:
				}
			}
			if refuse {
				return analytics.Computed[string]{}, analytics.ErrNeedFull
			}
			r.add("partial " + idsString(ids))
			return analytics.Computed[string]{Result: "partial", DataVersion: dataVersion, Customers: len(ids)}, nil
		},
		FullEvery: fullEvery,
		MaxShare:  maxShare,
		Total:     func(string) int { return 100 },
	})
	for _, f := range setup {
		f(r.e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := r.e.Start(ctx)
	t.Cleanup(func() {
		cancel()
		<-done
	})
	waitFor(t, "первый пересчёт", func() bool { return r.e.Latest() != nil })
	return r
}

func idsString(ids []int64) string {
	s := ""
	for _, id := range ids {
		s += string(rune('0'+id)) + ","
	}
	return s
}

func (r *partialRig) after(t *testing.T, version int64) *analytics.Calculation[string] {
	t.Helper()
	waitFor(t, "пересчёт", func() bool { return r.e.Latest().Version >= version })
	return r.e.Latest()
}

func TestPartialFirstFullThenPartial(t *testing.T) {
	r := newPartialRig(t, time.Minute, 0)
	r.bus.Publish(topicAccounts, []int64{3, 1})
	c := r.after(t, 2)
	if !c.Partial || c.Customers != 2 || c.DataVersion != 5 {
		t.Fatalf("итог %+v, ждали частичный на 2 клиента с версией данных 5", c.Computed)
	}
	if got := r.calls(); !slices.Equal(got, []string{"full", "partial 1,3,"}) {
		t.Errorf("вызовы %v", got)
	}
}

func TestPartialFullWithoutAccountsOrOnOtherTopic(t *testing.T) {
	r := newPartialRig(t, time.Minute, 0)
	r.bus.Publish(topicAccounts, []int64{}) // событие без клиентов — «всё»
	if c := r.after(t, 2); c.Partial {
		t.Error("пустой список клиентов должен дать полный пересчёт")
	}
	r.bus.Publish(topicData, int64(7)) // другая тема — «всё»
	if c := r.after(t, 3); c.Partial {
		t.Error("событие без клиентов должно дать полный пересчёт")
	}
}

func TestPartialNeedFullFallsBack(t *testing.T) {
	r := newPartialRig(t, time.Minute, 0)
	r.mu.Lock()
	r.refuse = true
	r.mu.Unlock()
	r.bus.Publish(topicAccounts, []int64{2})
	c := r.after(t, 2)
	if c.Partial || r.e.Errors() != 0 {
		t.Fatalf("отказ частичного: partial=%v, ошибок %d, ждали полный без ошибок", c.Partial, r.e.Errors())
	}
}

func TestPartialFullEveryAndShare(t *testing.T) {
	r := newPartialRig(t, time.Minute, 0.03)
	r.bus.Publish(topicAccounts, []int64{1, 2, 3, 4}) // 4 из 100 > 3 %
	if c := r.after(t, 2); c.Partial {
		t.Error("клиентов больше допустимой доли: пересчёт должен быть полным")
	}
	r.bus.Publish(topicAccounts, []int64{1, 2})
	if c := r.after(t, 3); !c.Partial {
		t.Error("2 из 100 в пределах доли: пересчёт должен быть частичным")
	}
	r.clk.Advance(2 * time.Minute) // прошёл период с последнего полного пересчёта
	r.bus.Publish(topicAccounts, []int64{1})
	if c := r.after(t, 4); c.Partial {
		t.Error("прошёл период FullEvery: пересчёт должен быть полным")
	}
	r.bus.Publish(topicAccounts, []int64{1})
	if c := r.after(t, 5); !c.Partial {
		t.Error("после полного период начался заново: пересчёт должен быть частичным")
	}
}

func TestPartialZeroFullEveryDisablesPartial(t *testing.T) {
	r := newPartialRig(t, 0, 0)
	r.bus.Publish(topicAccounts, []int64{1})
	if c := r.after(t, 2); c.Partial {
		t.Error("FullEvery = 0: частичный пересчёт выключен")
	}
}

// Прогон отменён новым событием: клиенты отменённого не теряются, а переходят в следующий.
func TestPartialCancelledRunKeepsAccounts(t *testing.T) {
	r := newPartialRig(t, time.Minute, 0)
	r.mu.Lock()
	r.gate = make(chan struct{})
	r.mu.Unlock()
	r.bus.Publish(topicAccounts, []int64{1})
	<-r.started // частичный пересчёт по клиенту 1 идёт и ждёт
	r.bus.Publish(topicAccounts, []int64{2})
	<-r.started // отменённый прогон завершился, следующий начался: он должен знать обоих
	close(r.gate)
	c := r.after(t, 2)
	if got := r.calls(); !slices.Equal(got, []string{"full", "cancelled", "partial 1,2,"}) {
		t.Errorf("вызовы %v, ждали отмену и один пересчёт по клиентам 1 и 2", got)
	}
	if c.Customers != 2 {
		t.Errorf("пересчитано %d клиентов, ждали 2", c.Customers)
	}
}

func TestEngineObserverSeesEachRecalc(t *testing.T) {
	var mu sync.Mutex
	var seen []bool
	r := newPartialRig(t, time.Minute, 0, func(e *analytics.Engine[string]) {
		e.WithObserver(func(c *analytics.Calculation[string]) {
			mu.Lock()
			seen = append(seen, c.Partial)
			mu.Unlock()
		})
	})
	r.bus.Publish(topicAccounts, []int64{1})
	r.after(t, 2)
	r.bus.Publish(topicData, int64(1))
	r.after(t, 3)
	mu.Lock()
	defer mu.Unlock()
	if !slices.Equal(seen, []bool{false, true, false}) {
		t.Errorf("наблюдатель увидел %v, ждали [false true false]: первый, частичный, полный", seen)
	}
}

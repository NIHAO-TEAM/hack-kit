package events_test

import (
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"testing"
	"time"

	"example.com/app/internal/platform/events"
)

func recv(t *testing.T, ch <-chan events.Event) events.Event {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(time.Second):
		t.Fatal("событие не пришло")
		return events.Event{}
	}
}

func TestSubscriberGetsEvent(t *testing.T) {
	bus := events.New()
	ch, cancel := bus.Subscribe("recalculated")
	defer cancel()

	bus.Publish("recalculated", int64(7))

	ev := recv(t, ch)
	if ev.Topic != "recalculated" || ev.Payload != int64(7) {
		t.Errorf("событие: %+v", ev)
	}
}

func TestOtherTopicAndCancel(t *testing.T) {
	bus := events.New()
	ch, cancel := bus.Subscribe("a")
	bus.Publish("b", nil)
	select {
	case ev := <-ch:
		t.Fatalf("пришло чужое событие: %+v", ev)
	default:
	}
	cancel()
	cancel() // повторная отмена безопасна
	bus.Publish("a", nil)
	if _, ok := <-ch; ok {
		t.Error("канал после cancel должен быть закрыт")
	}
}

func TestSlowSubscriberDoesNotBlock(t *testing.T) {
	bus := events.New()
	_, cancel := bus.Subscribe("a")
	defer cancel()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			bus.Publish("a", i)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("издатель завис на медленном подписчике")
	}
}

// Публикация из 8 горутин при одновременной подписке и отписке: ни паники «send on closed channel», ни гонки.
func TestPublishWhileSubscribeCancel(t *testing.T) {
	bus := events.New()
	stop := make(chan struct{})
	var pubs sync.WaitGroup
	for i := 0; i < 8; i++ {
		pubs.Add(1)
		go func() {
			defer pubs.Done()
			for {
				select {
				case <-stop:
					return
				default:
					bus.Publish("a", 1)
				}
			}
		}()
	}
	var subs sync.WaitGroup
	for g := 0; g < 10; g++ {
		subs.Add(1)
		go func() {
			defer subs.Done()
			for i := 0; i < 100; i++ {
				ch, cancel := bus.Subscribe("a")
				if i%2 == 0 {
					<-ch
				}
				cancel()
				for range ch { // канал закрыт: остаток буфера читается и заканчивается
				}
			}
		}()
	}
	subs.Wait()
	close(stop)
	pubs.Wait()
}

func metricValue(t *testing.T, bus *events.Bus, name, topic string) float64 {
	t.Helper()
	reg := prometheus.NewPedanticRegistry()
	reg.MustRegister(bus.Collector())
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, mf := range mfs {
		if mf.GetName() != name {
			continue
		}
		for _, m := range mf.GetMetric() {
			if m.GetLabel()[0].GetValue() != topic {
				continue
			}
			if m.GetCounter() != nil {
				return m.GetCounter().GetValue()
			}
			return m.GetGauge().GetValue()
		}
	}
	t.Fatalf("метрика %s{topic=%q} не найдена", name, topic)
	return 0
}

func TestSlowSubscriberCountsDrops(t *testing.T) {
	bus := events.New()
	ch, cancel := bus.SubscribeBuf("a", 4)
	defer cancel()
	for i := 0; i < 10; i++ {
		bus.Publish("a", i)
	}
	if got := metricValue(t, bus, "churn_events_dropped_total", "a"); got != 6 {
		t.Errorf("потеряно: %v, ожидалось 6", got)
	}
	// остались последние 4: теряется самое старое
	if ev := recv(t, ch); ev.Payload != 6 {
		t.Errorf("первое в буфере: %v, ожидалось 6", ev.Payload)
	}
}

func TestSubscribersGauge(t *testing.T) {
	bus := events.New()
	_, c1 := bus.Subscribe("a")
	_, c2 := bus.Subscribe("a")
	if got := metricValue(t, bus, "churn_events_subscribers", "a"); got != 2 {
		t.Errorf("подписчиков: %v, ожидалось 2", got)
	}
	c1()
	c2()
	if got := metricValue(t, bus, "churn_events_subscribers", "a"); got != 0 {
		t.Errorf("после отписки: %v, ожидалось 0", got)
	}
}

func TestSubscribeBufSize(t *testing.T) {
	bus := events.New()
	ch, cancel := bus.SubscribeBuf("a", 100)
	defer cancel()
	for i := 0; i < 100; i++ {
		bus.Publish("a", i)
	}
	if got := metricValue(t, bus, "churn_events_dropped_total", "a"); got != 0 {
		t.Errorf("потерь при буфере 100: %v", got)
	}
	if len(ch) != 100 {
		t.Errorf("в буфере %d", len(ch))
	}
	ch2, cancel2 := bus.SubscribeBuf("b", 0) // 0 → по умолчанию
	defer cancel2()
	if cap(ch2) != events.DefaultBuffer {
		t.Errorf("буфер по умолчанию: %d", cap(ch2))
	}
}

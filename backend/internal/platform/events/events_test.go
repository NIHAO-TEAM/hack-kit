package events_test

import (
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

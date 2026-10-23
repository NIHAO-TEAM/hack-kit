package events_test

import (
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

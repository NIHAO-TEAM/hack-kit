//go:build integration

package model_test

import (
	"context"
	"testing"
	"time"

	"example.com/app/internal/model"
	"example.com/app/internal/platform/db"
	"example.com/app/internal/platform/dbtest"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/risk"
)

func newStore(t *testing.T) (*model.Store, *events.Bus, string) {
	t.Helper()
	dsn := dbtest.New(t)
	ctx := context.Background()
	if err := db.Up(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	bus := events.New()
	return model.NewStore(pool, bus), bus, dsn
}

func TestStoreDefaultsThenSave(t *testing.T) {
	ctx := context.Background()
	store, bus, _ := newStore(t)
	ch, cancel := bus.Subscribe(model.TopicChanged)
	defer cancel()

	st, v, err := store.Current(ctx)
	if err != nil || v != 0 {
		t.Fatalf("пустая таблица: версия %d, ошибка %v", v, err)
	}
	if st.High != risk.Defaults().High || len(st.Weights) == 0 {
		t.Fatalf("ждали умолчания, получили %+v", st)
	}

	st.High = 80
	v, err = store.Save(ctx, st, "analyst")
	if err != nil || v != 1 {
		t.Fatalf("Save: версия %d, ошибка %v", v, err)
	}
	select {
	case ev := <-ch:
		if ev.Payload != int64(1) {
			t.Errorf("payload = %v", ev.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("model.changed не пришло")
	}
	got, v, err := store.Current(ctx)
	if err != nil || v != 1 || got.High != 80 {
		t.Fatalf("Current: %+v v=%d err=%v", got, v, err)
	}
}

func TestStoreInvalidKeepsVersion(t *testing.T) {
	ctx := context.Background()
	store, bus, _ := newStore(t)
	ch, cancel := bus.Subscribe(model.TopicChanged)
	defer cancel()

	bad := risk.Defaults()
	for id := range bad.Weights {
		bad.Weights[id] = 1.5
		break
	}
	if _, err := store.Save(ctx, bad, "analyst"); err == nil {
		t.Fatal("ждали ошибку для веса вне [0, 1]")
	}
	if _, v, _ := store.Current(ctx); v != 0 {
		t.Errorf("версия после ошибки = %d, ждали 0", v)
	}
	select {
	case ev := <-ch:
		t.Fatalf("событие при ошибке: %+v", ev)
	default:
	}
}

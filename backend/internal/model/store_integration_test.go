//go:build integration

package model_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"example.com/app/internal/model"
	"example.com/app/internal/platform/db"
	"example.com/app/internal/platform/dbtest"
	"example.com/app/internal/platform/events"
)

func newStore(t *testing.T) (*model.Store, *events.Bus) {
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
	validate := func(body []byte) error {
		if string(body) == `{"bad":true}` {
			return errors.New("неверный вес")
		}
		return nil
	}
	return model.NewStore(pool, bus, validate), bus
}

func TestStoreEmptyThenSave(t *testing.T) {
	ctx := context.Background()
	store, bus := newStore(t)
	ch, cancel := bus.Subscribe(model.TopicChanged)
	defer cancel()

	body, v, err := store.Current(ctx)
	if err != nil || v != 0 || body != nil {
		t.Fatalf("пустая таблица: тело %s, версия %d, ошибка %v", body, v, err)
	}

	v, err = store.Save(ctx, json.RawMessage(`{"high":80}`), "analyst")
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
	body, v, err = store.Current(ctx)
	if err != nil || v != 1 {
		t.Fatalf("Current: v=%d err=%v", v, err)
	}
	var got struct{ High float64 }
	if err := json.Unmarshal(body, &got); err != nil || got.High != 80 {
		t.Errorf("тело %s", body)
	}
}

func TestStoreInvalidKeepsVersion(t *testing.T) {
	ctx := context.Background()
	store, bus := newStore(t)
	ch, cancel := bus.Subscribe(model.TopicChanged)
	defer cancel()

	if _, err := store.Save(ctx, json.RawMessage(`{"bad":true}`), "analyst"); err == nil {
		t.Fatal("ждали ошибку проверки")
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

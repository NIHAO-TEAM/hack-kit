//go:build integration

package source_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"example.com/app/internal/apitest"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/source"
)

func TestReaderLoadsSeed(t *testing.T) {
	s := apitest.New(t)
	ctx := context.Background()
	r := source.NewReader(s.Pool)

	if _, _, err := r.Load(ctx); !errors.Is(err, source.ErrNoSnapshot) {
		t.Fatalf("без снимка: ждали ErrNoSnapshot, получили %v", err)
	}

	if err := source.OnStart(ctx, s.Pool, events.New(), s.Clock, true); err != nil {
		t.Fatal(err)
	}
	snap, version, err := r.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Accounts) != 10 || len(snap.Services) != 30 || len(snap.Transactions) != 100 {
		t.Errorf("счётчики %d/%d/%d", len(snap.Accounts), len(snap.Services), len(snap.Transactions))
	}
	if want := time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC); snap.AsOf == nil || !snap.AsOf.Equal(want) {
		t.Errorf("asOf = %v, ждали %v", snap.AsOf, want)
	}
	if version != 1 {
		t.Errorf("версия снимка = %d, ждали 1", version)
	}
}

func TestReaderLoadsStandSeed(t *testing.T) {
	t.Setenv("SEED_STAND", "true")
	s := apitest.New(t)
	ctx := context.Background()
	if err := source.OnStart(ctx, s.Pool, events.New(), s.Clock, true); err != nil {
		t.Fatal(err)
	}
	snap, _, err := source.NewReader(s.Pool).Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Accounts) != 16 || len(snap.Services) != 35 || len(snap.Transactions) != 105 {
		t.Errorf("счётчики %d/%d/%d", len(snap.Accounts), len(snap.Services), len(snap.Transactions))
	}
}

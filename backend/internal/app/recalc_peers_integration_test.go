//go:build integration

package app_test

import (
	"context"
	"testing"
	"time"

	"example.com/app/internal/app"
	"example.com/app/internal/platform/events"
)

// Снимок загружен через одну реплику (её шина), вторая реплика с отдельной шиной о нём узнаёт сама.
func TestReplicaSeesSnapshotLoadedViaPeer(t *testing.T) {
	t.Setenv("RECALC_PEER_POLL", "100ms")
	r := newRig(t, false) // пустая база: обе реплики стартуют без снимка
	a := r.start(t, r.Pool)

	bus2 := events.New()
	b := app.NewRecalc(r.Pool, bus2, r.Clock)
	ctx, cancel := context.WithCancel(context.Background())
	done := b.Start(ctx)
	t.Cleanup(func() {
		cancel()
		<-done
	})

	r.reload(t) // PUT идёт в API стенда, событие уходит в шину первой реплики
	waitLatest(t, a, "первая реплика посчитала снимок", func() bool { return a.Engine.Latest() != nil })
	deadline := time.Now().Add(15 * time.Second)
	for b.Engine.Latest() == nil {
		if time.Now().After(deadline) {
			t.Fatal("вторая реплика не узнала о снимке, загруженном через первую")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got, want := b.Engine.Latest().DataVersion, a.Engine.Latest().DataVersion; got != want {
		t.Errorf("версии данных: вторая %d, первая %d", got, want)
	}
}

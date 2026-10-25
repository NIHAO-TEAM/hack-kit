package source_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"example.com/app/internal/apitest"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/source"
)

func actions(t *testing.T, c *gen.ClientWithResponses, a ...gen.EmulatorAction) *gen.EmulatorActionsResponse {
	t.Helper()
	res, err := c.EmulatorActionsWithResponse(context.Background(), gen.EmulatorActionsRequest{Actions: a})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestEmulatorTickMovesTime(t *testing.T) {
	s := apitest.New(t)
	c := s.As(t, "analyst")
	first := put(t, c, organizers(t))
	events, cancel := s.Bus.Subscribe(source.TopicChanged)
	defer cancel()

	hours := 48
	res, err := c.EmulatorTickWithResponse(context.Background(), gen.EmulatorTickRequest{Hours: &hours})
	if err != nil || res.JSON200 == nil {
		t.Fatalf("tick: %v %v", err, res)
	}
	if res.JSON200.Version <= first.JSON200.Version {
		t.Errorf("версия %d → %d", first.JSON200.Version, res.JSON200.Version)
	}
	if want := first.JSON200.AsOf.Add(48 * time.Hour); !res.JSON200.AsOf.Equal(want) {
		t.Errorf("asOf %v, ждали %v", res.JSON200.AsOf, want)
	}
	if st := status(t, c); st.Version != res.JSON200.Version {
		t.Errorf("статус %d", st.Version)
	}
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Error("source.changed не опубликован")
	}

	zero := 0
	bad, err := c.EmulatorTickWithResponse(context.Background(), gen.EmulatorTickRequest{Hours: &zero})
	if err != nil || bad.StatusCode() != http.StatusBadRequest {
		t.Errorf("0 часов: %v %v", err, bad)
	}
}

func TestEmulatorActionsAndReset(t *testing.T) {
	s := apitest.New(t)
	c := s.As(t, "analyst")
	put(t, c, organizers(t))

	cents := int64(1234)
	res := actions(t, c, gen.EmulatorAction{ActionId: "top_up", CustomerId: 1, Cents: &cents}, gen.EmulatorAction{ActionId: "enable_autoprolong", CustomerId: 1})
	if res.JSON200 == nil {
		t.Fatalf("actions: %d %s", res.StatusCode(), res.Body)
	}
	var bal int64
	if err := s.Pool.QueryRow(context.Background(), `SELECT balance FROM accounts WHERE id = 1`).Scan(&bal); err != nil || bal != 83810+1234 {
		t.Errorf("баланс %d %v", bal, err)
	}
	var off int
	if err := s.Pool.QueryRow(context.Background(), `SELECT count(*) FROM services WHERE "ownerId" = 1 AND status <> 'deleted' AND NOT "autoProlong"`).Scan(&off); err != nil || off != 0 {
		t.Errorf("без автопродления осталось %d %v", off, err)
	}

	if bad := actions(t, c, gen.EmulatorAction{ActionId: "top_up", CustomerId: 99999, Cents: &cents}); bad.StatusCode() != http.StatusBadRequest {
		t.Errorf("неизвестный клиент: %d", bad.StatusCode())
	}

	reset := actions(t, c, gen.EmulatorAction{ActionId: "reset"})
	if reset.JSON200 == nil || reset.JSON200.Accounts < 10 {
		t.Fatalf("reset: %d %s", reset.StatusCode(), reset.Body)
	}
	if err := s.Pool.QueryRow(context.Background(), `SELECT balance FROM accounts WHERE id = 1`).Scan(&bal); err != nil || bal != 83810 {
		t.Errorf("после reset баланс %d %v", bal, err)
	}
	if want := time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC); !reset.JSON200.AsOf.Equal(want) {
		t.Errorf("asOf после reset %v", reset.JSON200.AsOf)
	}
}

func TestEmulatorPermissions(t *testing.T) {
	s := apitest.New(t)
	put(t, s.As(t, "analyst"), organizers(t))
	hours := 1
	for _, tc := range []struct {
		role string
		want int
	}{{"marketer", http.StatusOK}, {"analyst", http.StatusOK}, {"jury", http.StatusOK}, {"admin", http.StatusOK}} {
		c := s.As(t, tc.role)
		tick, err := c.EmulatorTickWithResponse(context.Background(), gen.EmulatorTickRequest{Hours: &hours})
		if err != nil || tick.StatusCode() != tc.want {
			t.Errorf("tick %s: %v %v", tc.role, err, tick)
		}
		act := actions(t, c, gen.EmulatorAction{ActionId: "reset"})
		if act.StatusCode() != tc.want {
			t.Errorf("actions %s: %d", tc.role, act.StatusCode())
		}
	}
	anon := s.Anonymous(t)
	if r, err := anon.EmulatorTickWithResponse(context.Background(), gen.EmulatorTickRequest{Hours: &hours}); err != nil || r.StatusCode() != http.StatusUnauthorized {
		t.Errorf("без сессии: %v %v", err, r)
	}
}

func TestEmulatorOrigin(t *testing.T) {
	s := apitest.New(t)
	c := s.As(t, "analyst")
	put(t, c, organizers(t))
	origin := func() gen.SourceStatusSource {
		t.Helper()
		st := status(t, c)
		if st.Source == nil {
			t.Fatal("источник не отдан")
		}
		return *st.Source
	}
	if got := origin(); got != gen.Api {
		t.Errorf("после PUT: %s", got)
	}
	if res, err := c.EmulatorTickWithResponse(context.Background(), gen.EmulatorTickRequest{}); err != nil || res.JSON200 == nil {
		t.Fatalf("tick: %v %v", err, res)
	}
	if got := origin(); got != gen.Emulator {
		t.Errorf("после tick: %s", got)
	}
	if res := actions(t, c, gen.EmulatorAction{ActionId: "reset"}); res.JSON200 == nil {
		t.Fatalf("reset: %d", res.StatusCode())
	}
	if got := origin(); got != gen.Seed {
		t.Errorf("после reset: %s", got)
	}
	put(t, c, organizers(t))
	if got := origin(); got != gen.Api {
		t.Errorf("после PUT: %s", got)
	}
}

//go:build integration

package source_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"example.com/app/internal/apitest"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/source"
)

// otherReplica — второй SourceAPI над той же базой: «другая реплика», у которой своей памяти о прогоне нет.
func otherReplica(s *apitest.Stand) *source.SourceAPI {
	return source.New(s.Pool, events.New(), s.Clock)
}

func streamOn(t *testing.T, a *source.SourceAPI) gen.StreamState {
	t.Helper()
	rec := httptest.NewRecorder()
	a.GetEmulatorStream(rec, httptest.NewRequest(http.MethodGet, "/api/v1/source/emulator/stream", nil))
	var st gen.StreamState
	if err := json.NewDecoder(rec.Body).Decode(&st); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestStreamStateVisibleFromAnotherReplica(t *testing.T) {
	s := apitest.New(t)
	s.StartRecalc(t)
	c := s.As(t, "analyst")
	if res := put(t, c, organizers(t)); res.JSON200 == nil {
		t.Fatalf("снимок: %s", res.Body)
	}
	b := otherReplica(s)
	if st := streamOn(t, b); st.State != gen.Idle {
		t.Fatalf("до запуска: %s", st.State)
	}

	res := startStream(t, c, 300, 100, 1) // ≈3 с, чтобы застать «идёт»
	if res.JSON202 == nil {
		t.Fatalf("запуск: %d %s", res.StatusCode(), res.Body)
	}
	runID := res.JSON202.RunId

	var sawRunning bool
	deadline := time.Now().Add(2 * time.Minute)
	var st gen.StreamState
	for time.Now().Before(deadline) {
		st = streamOn(t, b)
		if st.RunId == nil || *st.RunId != runID {
			t.Fatalf("другая реплика не видит прогон %s: %+v", runID, st)
		}
		if st.State == gen.Running {
			sawRunning = true
		} else {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !sawRunning {
		t.Error("другая реплика не увидела состояние running")
	}
	if st.State != gen.Done || st.Report == nil || st.Report.Applied != 300 {
		t.Fatalf("итог на другой реплике: %s, отчёт %+v, ошибка %v", st.State, st.Report, st.Error)
	}
}

func TestStreamStateOfDeadReplicaIsFailed(t *testing.T) {
	s := apitest.New(t)
	runID := "dead-run"
	raw, _ := json.Marshal(gen.StreamState{State: gen.Running, RunId: &runID, Events: 10000, Applied: 4000})
	if _, err := s.Pool.Exec(context.Background(),
		`INSERT INTO stream_runs (run_id, state, updated_at) VALUES ($1, $2, now() - interval '1 minute')`, runID, raw); err != nil {
		t.Fatal(err)
	}
	b := otherReplica(s)
	st := streamOn(t, b)
	if st.State != gen.Failed || st.Error == nil || st.RunId == nil || *st.RunId != runID {
		t.Fatalf("ожидали failed с ошибкой, получили %+v", st)
	}
	if again := streamOn(t, b); again.State != gen.Failed {
		t.Errorf("повторный опрос: %s", again.State)
	}
}

func TestStreamStateWithLiveLeaderStaysRunning(t *testing.T) {
	s := apitest.New(t)
	runID := "slow-run"
	raw, _ := json.Marshal(gen.StreamState{State: gen.Running, RunId: &runID, Events: 10000})
	if _, err := s.Pool.Exec(context.Background(),
		`INSERT INTO stream_runs (run_id, state, updated_at) VALUES ($1, $2, now() - interval '1 minute')`, runID, raw); err != nil {
		t.Fatal(err)
	}
	// Ведущая реплика жива: держит блокировку прогона на своём соединении.
	conn, err := s.Pool.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()
	if _, err := conn.Exec(context.Background(), "SELECT pg_advisory_lock($1)", int64(7_405_120_001)); err != nil {
		t.Fatal(err)
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", int64(7_405_120_001)) //nolint:errcheck
	if st := streamOn(t, otherReplica(s)); st.State != gen.Running {
		t.Fatalf("при живой ведущей реплике ожидали running, получили %s", st.State)
	}
}

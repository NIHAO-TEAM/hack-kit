package source_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"example.com/app/internal/apitest"
	"example.com/app/internal/platform/api/gen"
)

func startStream(t *testing.T, c *gen.ClientWithResponses, events, rate int, seed int64) *gen.StartEmulatorStreamResponse {
	t.Helper()
	res, err := c.StartEmulatorStreamWithResponse(context.Background(), gen.StreamRequest{Events: &events, RatePerSecond: &rate, Seed: &seed})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// waitStream ждёт конца прогона и возвращает его состояние.
func waitStream(t *testing.T, c *gen.ClientWithResponses) gen.StreamState {
	t.Helper()
	deadline := time.Now().Add(3 * time.Minute)
	for time.Now().Before(deadline) {
		res, err := c.GetEmulatorStreamWithResponse(context.Background())
		if err != nil || res.JSON200 == nil {
			t.Fatalf("состояние потока: %v %v", err, res)
		}
		if res.JSON200.State != gen.Running {
			return *res.JSON200
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("прогон не закончился за 3 минуты")
	return gen.StreamState{}
}

func TestStreamRunsAndReportsConsistency(t *testing.T) {
	s := apitest.New(t)
	s.StartRecalc(t)
	c := s.As(t, "analyst")
	if res := put(t, c, organizers(t)); res.JSON200 == nil {
		t.Fatalf("снимок: %s", res.Body)
	}

	progress, cancel := s.Bus.Subscribe("source.stream")
	defer cancel()
	res := startStream(t, c, 300, 0, 1)
	if res.JSON202 == nil || res.JSON202.RunId == "" {
		t.Fatalf("запуск: %d %s", res.StatusCode(), res.Body)
	}
	st := waitStream(t, c)
	if st.State != gen.Done || st.Report == nil {
		t.Fatalf("состояние %s, ошибка %v", st.State, st.Error)
	}
	r := st.Report
	if r.Events != 300 || r.Applied != 300 || r.Failed != 0 {
		t.Errorf("события %d, применено %d, не применено %d", r.Events, r.Applied, r.Failed)
	}
	if !r.Consistency.Ok || r.Consistency.DiffCents != 0 {
		t.Errorf("согласованность: %+v", r.Consistency)
	}
	if r.Recalcs < 1 || r.Recalcs > 300 {
		t.Errorf("пересчётов %d", r.Recalcs)
	}
	if r.ApplyP50Ms <= 0 || r.ApplyP95Ms < r.ApplyP50Ms || r.ApplyMaxMs < r.ApplyP95Ms {
		t.Errorf("задержки применения p50 %v p95 %v max %v", r.ApplyP50Ms, r.ApplyP95Ms, r.ApplyMaxMs)
	}
	if r.ApplyP99Ms == nil || *r.ApplyP99Ms < r.ApplyP95Ms || *r.ApplyP99Ms > r.ApplyMaxMs {
		t.Errorf("p99 применения %v при p95 %v и max %v", r.ApplyP99Ms, r.ApplyP95Ms, r.ApplyMaxMs)
	}
	if r.QueueLagP95Ms == nil || *r.QueueLagP95Ms <= 0 {
		t.Errorf("задержка до очереди %v", r.QueueLagP95Ms)
	}
	if r.EventsPerSecond <= 0 || r.DurationMs <= 0 {
		t.Errorf("скорость %v, длительность %d", r.EventsPerSecond, r.DurationMs)
	}
	if r.LossBeforeCents == r.LossAfterCents {
		t.Errorf("потеря не изменилась: %d", r.LossAfterCents)
	}
	if sum(r.LevelsBefore) != sum(r.LevelsAfter) {
		t.Errorf("число клиентов до %d, после %d", sum(r.LevelsBefore), sum(r.LevelsAfter))
	}

	// Ход ушёл в шину, последний кадр — готово.
	var last gen.StreamState
	for drained := false; !drained; {
		select {
		case ev := <-progress:
			last = ev.Payload.(gen.StreamState)
		default:
			drained = true
		}
	}
	if last.State != gen.Done || last.Applied != 300 {
		t.Errorf("последний кадр хода: %s, применено %d", last.State, last.Applied)
	}
}

func sum(l gen.LevelCounts) int { return l.High + l.Medium + l.Low + l.Unscored + l.Lost }

func TestStreamPermissionsAndLimits(t *testing.T) {
	s := apitest.New(t)
	s.StartRecalc(t)
	for role, want := range map[string]int{"marketer": http.StatusForbidden} {
		if res := startStream(t, s.As(t, role), 100, 0, 1); res.StatusCode() != want {
			t.Errorf("%s: запуск %d, ждали %d", role, res.StatusCode(), want)
		}
		got, err := s.As(t, role).GetEmulatorStreamWithResponse(context.Background())
		if err != nil || got.StatusCode() != want {
			t.Errorf("%s: состояние %v %v", role, got, err)
		}
	}
	if res := startStream(t, s.Anonymous(t), 100, 0, 1); res.StatusCode() != http.StatusUnauthorized {
		t.Errorf("без сессии: %d", res.StatusCode())
	}
	c := s.As(t, "jury")
	for _, tc := range []struct{ events, rate int }{{99, 0}, {100001, 0}, {100, -1}, {100, 20001}} {
		if res := startStream(t, c, tc.events, tc.rate, 1); res.StatusCode() != http.StatusBadRequest {
			t.Errorf("events %d rate %d: %d, ждали 400", tc.events, tc.rate, res.StatusCode())
		}
	}
	idle, err := c.GetEmulatorStreamWithResponse(context.Background())
	if err != nil || idle.JSON200 == nil || idle.JSON200.State != gen.Idle {
		t.Errorf("до первого запуска: %v %v", idle, err)
	}
}

func TestStreamAllowsOnlyOneRun(t *testing.T) {
	s := apitest.New(t)
	s.StartRecalc(t)
	c := s.As(t, "analyst")
	if res := startStream(t, c, 100, 20, 1); res.JSON202 == nil { // около 5 секунд
		t.Fatalf("первый запуск: %d %s", res.StatusCode(), res.Body)
	}
	if res := startStream(t, c, 100, 0, 1); res.StatusCode() != http.StatusConflict {
		t.Errorf("второй запуск: %d, ждали 409", res.StatusCode())
	}
	if st := waitStream(t, c); st.State != gen.Done {
		t.Errorf("первый прогон: %s %v", st.State, st.Error)
	}
	if res := startStream(t, c, 100, 0, 1); res.JSON202 == nil {
		t.Errorf("после конца прогона: %d %s", res.StatusCode(), res.Body)
	}
	waitStream(t, c)
}

func TestStreamMetrics(t *testing.T) {
	s := apitest.New(t)
	s.StartRecalc(t)
	c := s.As(t, "analyst")
	startStream(t, c, 100, 0, 5)
	waitStream(t, c)

	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, s.URL+"/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+apitest.MetricsToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	body := string(raw)
	for _, want := range []string{
		`churn_emulator_stream_events_total{result="applied"}`,
		"churn_emulator_stream_apply_seconds_bucket",
		"churn_emulator_stream_apply_seconds_count",
		"churn_ingest_batch_size_count",
		"churn_ingest_batch_seconds_count",
		"churn_ingest_queue_depth",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("в /metrics нет %s", want)
		}
	}
}

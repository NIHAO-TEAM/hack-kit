//go:build integration

package source_test

import (
	"os"
	"strconv"
	"testing"

	"example.com/app/internal/apitest"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/tools/snapgen"
)

func envInt(t *testing.T, name string) int {
	t.Helper()
	n, err := strconv.Atoi(os.Getenv(name))
	if err != nil || n <= 0 {
		t.Skipf("замер выключен: задайте %s", name)
	}
	return n
}

// Замер потока на большом снимке. Включается переменными INGEST_BENCH_ACCOUNTS (клиентов) и
// INGEST_BENCH_EVENTS (событий); скорость «как можно быстрее». Числа идут в docs/load/.
func TestStreamBench(t *testing.T) {
	accounts := envInt(t, "INGEST_BENCH_ACCOUNTS")
	events := envInt(t, "INGEST_BENCH_EVENTS")
	body, err := snapgen.Generate(snapgen.Options{Accounts: accounts, Seed: 7})
	if err != nil {
		t.Fatal(err)
	}
	s := apitest.New(t)
	s.StartRecalc(t)
	c := s.As(t, "jury")
	if res := put(t, c, body); res.JSON200 == nil {
		t.Fatalf("снимок: %d %s", res.StatusCode(), res.Body)
	}
	if res := startStream(t, c, events, 0, 1); res.JSON202 == nil {
		t.Fatalf("запуск: %d %s", res.StatusCode(), res.Body)
	}
	st := waitStream(t, c)
	if st.State != gen.Done || st.Report == nil {
		t.Fatalf("состояние %s, ошибка %v", st.State, st.Error)
	}
	r := st.Report
	t.Logf("клиентов %d, событий %d, применено %d, не применено %d", accounts, r.Events, r.Applied, r.Failed)
	t.Logf("запись %d мс, %.1f событий/с; p50 %.2f p95 %.2f max %.2f мс; пересчётов %d", r.DurationMs, r.EventsPerSecond, r.ApplyP50Ms, r.ApplyP95Ms, r.ApplyMaxMs, r.Recalcs)
	t.Logf("согласованность %+v", r.Consistency)
	if !r.Consistency.Ok || r.Consistency.DiffCents != 0 {
		t.Errorf("согласованность: %+v", r.Consistency)
	}
}

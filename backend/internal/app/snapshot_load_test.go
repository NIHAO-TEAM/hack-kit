//go:build integration && load

package app_test

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"example.com/app/internal/apitest"
	"example.com/app/internal/auth"
	"example.com/app/tools/snapgen"
)

// peakRSSKB — пик резидентной памяти процесса (VmHWM) в КБ; 0, если /proc недоступен.
func peakRSSKB() int {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "VmHWM:") {
			f := strings.Fields(l)
			n, _ := strconv.Atoi(f[1])
			return n
		}
	}
	return 0
}

// Замер большого снимка (docs/load): go test -tags 'integration load' -run TestLargeSnapshotLoad -v ./internal/app
// Сервер, пересчёт и клиент — в одном процессе, поэтому память общая.
func TestLargeSnapshotLoad(t *testing.T) {
	accounts := 10000
	if v, err := strconv.Atoi(os.Getenv("LOAD_ACCOUNTS")); err == nil && v > 0 {
		accounts = v
	}
	body, err := snapgen.Generate(snapgen.Options{Accounts: accounts, Seed: 11})
	if err != nil {
		t.Fatal(err)
	}
	r := newRig(t, false)
	r.StartRecalc(t)
	ctx := context.Background()
	marketer := r.As(t, auth.RoleMarketer)
	loader := r.WithBearer(t, apitest.SourceToken)

	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	start := time.Now()
	put, err := loader.PutSnapshotWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
	if err != nil || put.JSON200 == nil {
		t.Fatalf("PUT: %v %d %s", err, put.StatusCode(), put.Body)
	}
	putTook := time.Since(start)

	for {
		res, err := marketer.GetSummaryWithResponse(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if res.JSON200 != nil && res.JSON200.Version >= 1 {
			break
		}
		if time.Since(start) > 5*time.Minute {
			t.Fatal("пересчёт не завершился за 5 минут")
		}
		time.Sleep(20 * time.Millisecond)
	}
	total := time.Since(start)

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	t.Logf("аккаунтов %d, услуг %d, платежей %d; тело %.1f МБ", put.JSON200.Accounts, put.JSON200.Services, put.JSON200.Transactions, float64(len(body))/(1<<20))
	t.Logf("PUT: %v; от PUT до готовой сводки: %v", putTook.Round(time.Millisecond), total.Round(time.Millisecond))
	t.Logf("куча после: %.0f МБ (до: %.0f МБ), всего выделено за прогон: %.0f МБ, пик RSS процесса: %.0f МБ",
		float64(after.HeapInuse)/(1<<20), float64(before.HeapInuse)/(1<<20),
		float64(after.TotalAlloc-before.TotalAlloc)/(1<<20), float64(peakRSSKB())/1024)
}

//go:build integration

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/app/internal/apitest"
	"example.com/app/tools/snapgen"
)

// heapBytes — живые и ещё не собранные объекты кучи (дёшево, без остановки мира).
func heapBytes() uint64 {
	s := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	metrics.Read(s)
	return s[0].Value.Uint64()
}

// peakHeapDuring возвращает кучу до вызова f (после сборки мусора) и её пик за время f.
func peakHeapDuring(f func()) (before, peak uint64) {
	runtime.GC()
	before = heapBytes()
	peak = before
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if h := heapBytes(); h > peak {
				peak = h
			}
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}()
	f()
	close(stop)
	wg.Wait()
	return before, peak
}

func writeSnapshotFile(t *testing.T, path string, accounts int) int {
	t.Helper()
	body, err := snapgen.Generate(snapgen.Options{Accounts: accounts, Seed: 11})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	return len(body)
}

// putGrowth грузит снимок на n аккаунтов и возвращает рост кучи за PUT (пик минус до) в МБ.
// Пересчёт не запущен, поэтому в кучу попадает только сам приём (и клиент теста).
func putGrowth(t *testing.T, accounts int) float64 {
	t.Helper()
	// Тело уходит на диск и читается оттуда: в куче теста оно не лежит и «до» от размера снимка не зависит.
	path := filepath.Join(t.TempDir(), "snapshot.json")
	size := writeSnapshotFile(t, path, accounts)
	// Мусор копится до доли живой кучи; малая доля делает пик ближе к живой памяти приёма.
	defer debug.SetGCPercent(debug.SetGCPercent(20))
	r := newRig(t, false)
	loader := r.WithBearer(t, apitest.SourceToken)
	var took time.Duration
	before, peak := peakHeapDuring(func() {
		f, err := os.Open(path)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = f.Close() }()
		start := time.Now()
		res, err := loader.PutSnapshotWithBodyWithResponse(context.Background(), "application/json", f)
		if err != nil || res.JSON200 == nil {
			t.Errorf("PUT: %v %d %s", err, res.StatusCode(), res.Body)
			return
		}
		took = time.Since(start)
		t.Logf("аккаунтов %d, услуг %d, платежей %d; тело %.1f МБ", res.JSON200.Accounts, res.JSON200.Services, res.JSON200.Transactions, float64(size)/(1<<20))
	})
	growth := float64(peak-before) / (1 << 20)
	t.Logf("PUT %v; куча до %.0f МБ, пик %.0f МБ, рост %.0f МБ", took.Round(time.Millisecond), float64(before)/(1<<20), float64(peak)/(1<<20), growth)
	return growth
}

// Границы роста кучи за PUT (МБ). До потоковой загрузки было 296 МБ на 100k и 852 МБ на 300k
// (docs/load/2026-10-24-snapshot-stream.md). Сейчас растёт только множество email аккаунтов:
// идентификаторы лежат в битовых картах, записи в памяти не копятся.
const (
	maxGrowth100kMB = 32
	maxGrowth300kMB = 64
)

// Пик памяти приёма не зависит от числа услуг и платежей и ограничен записанной границей.
// 300k аккаунтов (≈ 1,2 млн платежей, 327 МБ тела) — при LOAD_300K=1: около минуты и 1 ГБ на генерацию.
func TestSnapshotIngestPeakMemory(t *testing.T) {
	if g := putGrowth(t, 100000); g > maxGrowth100kMB {
		t.Errorf("рост кучи на 100k аккаунтов %.0f МБ, граница %d МБ", g, maxGrowth100kMB)
	}
	if os.Getenv("LOAD_300K") != "1" {
		t.Skip("300k аккаунтов: задайте LOAD_300K=1")
	}
	if g := putGrowth(t, 300000); g > maxGrowth300kMB {
		t.Errorf("рост кучи на 300k аккаунтов %.0f МБ, граница %d МБ", g, maxGrowth300kMB)
	}
}

func tableCounts(t *testing.T, r *rig) (accounts, services, transactions, version int64) {
	t.Helper()
	err := r.Pool.QueryRow(context.Background(),
		`SELECT (SELECT count(*) FROM accounts), (SELECT count(*) FROM services), (SELECT count(*) FROM transactions),
		        COALESCE((SELECT max(version) FROM snapshots), 0)`).Scan(&accounts, &services, &transactions, &version)
	if err != nil {
		t.Fatal(err)
	}
	return
}

func putRaw(t *testing.T, r *rig, body []byte) (status int, text string) {
	t.Helper()
	res, err := r.WithBearer(t, apitest.SourceToken).PutSnapshotWithBodyWithResponse(context.Background(), "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode(), string(res.Body)
}

// topLevel режет тело снимка на значения ключей верхнего уровня, чтобы собрать их в другом порядке.
func topLevel(t *testing.T, body []byte) map[string]json.RawMessage {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func joinKeys(m map[string]json.RawMessage, order ...string) []byte {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range order {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%q:%s", k, m[k])
	}
	b.WriteByte('}')
	return b.Bytes()
}

// Массивы снимка принимаются в любом порядке ключей: те, на которые ссылается внешний ключ,
// ждут записи своих владельцев; итог и версия те же, что при обычном порядке.
func TestSnapshotIngestAnyKeyOrder(t *testing.T) {
	body, err := snapgen.Generate(snapgen.Options{Accounts: 300, Seed: 5})
	if err != nil {
		t.Fatal(err)
	}
	parts := topLevel(t, body)
	var wantA, wantS, wantT int64
	for i, order := range [][]string{
		{"asOf", "accounts", "services", "transactions"},
		{"transactions", "services", "accounts", "asOf"},
		{"services", "transactions", "asOf", "accounts"},
		{"transactions", "accounts", "services"},
	} {
		r := newRig(t, false)
		if code, text := putRaw(t, r, joinKeys(parts, order...)); code != 200 {
			t.Fatalf("порядок %v: %d %s", order, code, text)
		}
		a, s, tx, v := tableCounts(t, r)
		if i == 0 {
			wantA, wantS, wantT = a, s, tx
		}
		if a != wantA || s != wantS || tx != wantT || v != 1 {
			t.Errorf("порядок %v: строк %d/%d/%d версия %d, ждали %d/%d/%d версия 1", order, a, s, tx, v, wantA, wantS, wantT)
		}
	}
}

// Битая запись посреди потока откатывает всю загрузку: таблицы и версия прежние,
// ответ 422 называет запись; оборванное тело — 400 и тоже без следов.
func TestSnapshotIngestRollsBackMidStream(t *testing.T) {
	r := newRig(t, true) // сид: 10 аккаунтов, 30 услуг, 100 платежей
	a0, s0, t0, v0 := tableCounts(t, r)

	body, err := snapgen.Generate(snapgen.Options{Accounts: 12000, Seed: 3}) // ≈ 48 000 платежей
	if err != nil {
		t.Fatal(err)
	}
	// Битым становится платёж из середины массива: тип с неизвестным значением.
	marker := []byte(`"type":"order"`)
	total := bytes.Count(body, marker)
	if total < 1000 {
		t.Fatalf("в снимке мало платежей типа order: %d", total)
	}
	pos := 0
	for n := 0; n < total/2; n++ {
		pos += bytes.Index(body[pos:], marker) + len(marker)
	}
	body = append(append(append([]byte{}, body[:pos-len(marker)]...), []byte(`"type":"bogus"`)...), body[pos:]...)
	code, text := putRaw(t, r, body)
	if code != 422 || !strings.Contains(text, ".type") {
		t.Fatalf("битая запись: %d %s", code, text)
	}
	if a, s, x, v := tableCounts(t, r); a != a0 || s != s0 || x != t0 || v != v0 {
		t.Errorf("после 422 строк %d/%d/%d версия %d, было %d/%d/%d версия %d", a, s, x, v, a0, s0, t0, v0)
	}

	code, text = putRaw(t, r, body[:len(body)/2]) // тело оборвано посреди массива
	if code != 400 {
		t.Fatalf("оборванное тело: %d %s", code, text)
	}
	if a, s, x, v := tableCounts(t, r); a != a0 || s != s0 || x != t0 || v != v0 {
		t.Errorf("после 400 строк %d/%d/%d версия %d, было %d/%d/%d версия %d", a, s, x, v, a0, s0, t0, v0)
	}
}

// Метрики этапов сохранены, добавлен пик кучи загрузки.
func TestSnapshotIngestMetrics(t *testing.T) {
	body, err := os.ReadFile("../source/seed/organizers.json")
	if err != nil {
		t.Fatal(err)
	}
	r := newRig(t, false)
	if code, text := putRaw(t, r, body); code != 200 {
		t.Fatalf("PUT: %d %s", code, text)
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, r.URL+"/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+apitest.MetricsToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	for _, stage := range []string{"parse", "validate", "lock", "truncate", "copy_accounts", "copy_services", "copy_transactions", "sequences", "commit"} {
		if !strings.Contains(string(raw), `churn_snapshot_ingest_stage_seconds_count{stage="`+stage+`"}`) {
			t.Errorf("в /metrics нет этапа %s", stage)
		}
	}
	m := regexp.MustCompile(`(?m)^churn_snapshot_ingest_peak_heap_bytes ([0-9.e+]+)$`).FindSubmatch(raw)
	if m == nil {
		t.Fatalf("в /metrics нет churn_snapshot_ingest_peak_heap_bytes")
	}
	if v, _ := strconv.ParseFloat(string(m[1]), 64); v <= 0 {
		t.Errorf("пик кучи %s, ждали больше нуля", m[1])
	}
}

// Клиент, который начал отдавать тело и замолчал, не держит таблицы дольше срока на тело:
// ответ 400, прежние данные и версия на месте.
func TestSnapshotIngestBodyTimeout(t *testing.T) {
	t.Setenv("SNAPSHOT_BODY_TIMEOUT_SEC", "2")
	r := newRig(t, true)
	a0, s0, t0, v0 := tableCounts(t, r)

	pr, pw := io.Pipe()
	go func() {
		_, _ = pw.Write([]byte(`{"accounts":[{"id":1,"email":"slow@x","balance":0,"withdrawBalance":0,"totalReplenished":0,"bonusBalance":0,"bonusState":"locked"},`))
		// дальше тишина: клиент завис
	}()
	t.Cleanup(func() { _ = pw.Close() })

	start := time.Now()
	res, err := r.WithBearer(t, apitest.SourceToken).PutSnapshotWithBodyWithResponse(context.Background(), "application/json", pr)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode() != 400 {
		t.Fatalf("зависшее тело: %d %s, ждали 400", res.StatusCode(), res.Body)
	}
	if took := time.Since(start); took > 10*time.Second {
		t.Errorf("ответ через %v, срок на тело 2 с", took)
	}
	if a, s, x, v := tableCounts(t, r); a != a0 || s != s0 || x != t0 || v != v0 {
		t.Errorf("после таймаута строк %d/%d/%d версия %d, было %d/%d/%d версия %d", a, s, x, v, a0, s0, t0, v0)
	}
}

package main

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"example.com/app/internal/platform/httpx"
)

// startServe поднимает serve на свободном порту; done получает результат остановки.
func startServe(t *testing.T, h http.Handler, drain, timeout time.Duration) (base string, lc *httpx.Lifecycle, stop context.CancelFunc, done chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	lc = httpx.NewLifecycle()
	srv := newServer(ln.Addr().String(), h, lc)
	srv.ReadTimeout = 200 * time.Millisecond // короче жизни SSE в тесте
	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	done = make(chan error, 1)
	go func() { done <- serve(ctx, srv, ln, lc, drain, timeout) }()
	return "http://" + ln.Addr().String(), lc, stop, done
}

// Медленный запрос дорабатывает, SSE закрывается с retry, /readyz отвечает 503 до закрытия.
func TestServeGracefulShutdown(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/", newMux())
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_, _ = io.WriteString(w, "done")
	})
	const drain = 300 * time.Millisecond
	base, lc, stop, done := startServe(t, mux, drain, 5*time.Second)

	// SSE живёт дольше ReadTimeout (200 мс).
	sse, err := http.Get(base + "/api/v1/events")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sse.Body.Close() }()
	r := bufio.NewReader(sse.Body)
	if line, _ := r.ReadString('\n'); !strings.HasPrefix(line, ": connected") {
		t.Fatalf("первая строка потока %q", line)
	}
	time.Sleep(400 * time.Millisecond)

	slowRes := make(chan string, 1)
	go func() {
		resp, err := http.Get(base + "/slow")
		if err != nil {
			slowRes <- "ошибка: " + err.Error()
			return
		}
		defer func() { _ = resp.Body.Close() }()
		b, _ := io.ReadAll(resp.Body)
		slowRes <- string(b)
	}()
	time.Sleep(100 * time.Millisecond) // запрос уже в работе

	started := time.Now()
	stop()
	time.Sleep(drain / 3) // идёт пауза: сервер ещё принимает, но не готов
	resp, err := http.Get(base + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("/readyz во время остановки: %d", resp.StatusCode)
	}

	if got := <-slowRes; got != "done" {
		t.Fatalf("медленный запрос: %q", got)
	}
	rest, _ := io.ReadAll(r) // поток закрыт сервером
	if !strings.Contains(string(rest), "retry:") {
		t.Fatalf("поток закрылся без retry: %q", rest)
	}
	if err := <-done; err != nil {
		t.Fatalf("остановка: %v", err)
	}
	if d := time.Since(started); d > 3*time.Second {
		t.Fatalf("остановка заняла %s", d)
	}
	if n := lc.InflightAtShutdown(); n < 2 { // медленный запрос и SSE
		t.Fatalf("запросов в работе на сигнале: %d", n)
	}
}

// /readyz идёт мимо обработчика приложения (без сессии) и отвечает 200, пока всё готово.
func TestReadyzServed(t *testing.T) {
	base, _, stop, done := startServe(t, newHandler(), 0, time.Second)
	resp, err := http.Get(base + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/readyz: %d", resp.StatusCode)
	}
	stop()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestEnvDuration(t *testing.T) {
	t.Setenv("SHUTDOWN_DRAIN", "")
	if d, err := envDuration("SHUTDOWN_DRAIN", defaultDrain); err != nil || d != defaultDrain {
		t.Fatalf("умолчание: %v %v", d, err)
	}
	t.Setenv("SHUTDOWN_DRAIN", "2s")
	if d, _ := envDuration("SHUTDOWN_DRAIN", defaultDrain); d != 2*time.Second {
		t.Fatalf("2s: %v", d)
	}
	t.Setenv("SHUTDOWN_DRAIN", "abc")
	if _, err := envDuration("SHUTDOWN_DRAIN", defaultDrain); err == nil {
		t.Fatal("ждали ошибку")
	}
}

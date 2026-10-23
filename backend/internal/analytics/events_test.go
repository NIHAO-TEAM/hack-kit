package analytics_test

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"example.com/app/internal/analytics"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
)

func TestGetEventsStreamsRecalculated(t *testing.T) {
	bus := events.New()
	api := analytics.New(nil, bus, clock.NewFake(time.Now()))
	srv := httptest.NewServer(http.HandlerFunc(api.GetEvents))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if got := resp.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q", got)
	}
	if resp.Header.Get("X-Accel-Buffering") != "no" || resp.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("заголовки: %v", resp.Header)
	}

	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	deadline := time.After(2 * time.Second)
	// Поток открыт (комментарий connected) — подписка уже оформлена, событие не потеряется.
	if l := <-lines; !strings.HasPrefix(l, ":") {
		t.Fatalf("первая строка = %q", l)
	}
	bus.Publish(analytics.TopicRecalculated, int64(5))
	var got []string
	for len(got) < 2 {
		select {
		case l := <-lines:
			if l != "" {
				got = append(got, l)
			}
		case <-deadline:
			t.Fatalf("не дождались события, получено: %v", got)
		}
	}
	if got[0] != "event: recalculated" || !strings.Contains(got[1], `"version":5`) {
		t.Fatalf("событие: %v", got)
	}
}

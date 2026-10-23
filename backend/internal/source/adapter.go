package source

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Адаптер REST-биллинга: «адаптер под наш контракт: формат API aéza неизвестен».
// Забирает снимок той же формы, что data.json, и пишет его тем же путём, что PUT /source/snapshot.

const (
	defaultPollSeconds = 60
	requestTimeout     = 10 * time.Second
	maxBackoff         = 5 * time.Minute
)

// AdapterConfig — настройки опроса.
type AdapterConfig struct {
	URL      string
	Token    string
	Interval time.Duration
}

// AdapterConfigFromEnv читает BILLING_URL, BILLING_TOKEN и BILLING_POLL_SECONDS.
// Пустой BILLING_URL — опрос не настроен, возвращается nil.
func AdapterConfigFromEnv() *AdapterConfig {
	url := strings.TrimSpace(os.Getenv("BILLING_URL"))
	if url == "" {
		return nil
	}
	secs := defaultPollSeconds
	if v, err := strconv.Atoi(os.Getenv("BILLING_POLL_SECONDS")); err == nil && v > 0 {
		secs = v
	}
	return &AdapterConfig{URL: url, Token: os.Getenv("BILLING_TOKEN"), Interval: time.Duration(secs) * time.Second}
}

// PollStatus — состояние опроса для экрана «Источник».
type PollStatus struct {
	Paused      bool
	Interval    time.Duration
	LastOKAt    *time.Time
	LastError   string
	ErrorsTotal int64
}

// backoff — пауза до следующего опроса: интервал, после каждого сбоя подряд вдвое больше,
// но не больше maxBackoff (интервал, превышающий потолок, остаётся как есть).
func backoff(interval time.Duration, fails int) time.Duration {
	d := interval
	for i := 0; i < fails && d < maxBackoff; i++ {
		d *= 2
	}
	if d > maxBackoff && interval < maxBackoff {
		d = maxBackoff
	}
	return d
}

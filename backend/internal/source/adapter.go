package source

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
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

// Adapter опрашивает REST-биллинг.
type Adapter struct {
	pool *pgxpool.Pool
	bus  *events.Bus
	clk  clock.Clock
	cfg  AdapterConfig
	http *http.Client

	maxBytes int64

	mu       sync.Mutex
	paused   bool
	lastHash [sha256.Size]byte
	hasHash  bool
	lastOK   *time.Time
	lastErr  string
	fails    int // сбоев подряд, для задержки между повторами
	errCount int64

	errorsTotal prometheus.Counter
	pollsTotal  *prometheus.CounterVec
}

// NewAdapter создаёт адаптер; запускает его Run.
func NewAdapter(pool *pgxpool.Pool, bus *events.Bus, clk clock.Clock, cfg AdapterConfig) *Adapter {
	return &Adapter{
		pool: pool, bus: bus, clk: clk, cfg: cfg,
		http:     &http.Client{Timeout: requestTimeout},
		maxBytes: defaultMaxBytes,
		errorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "churn_billing_poll_errors_total",
			Help: "Сбои опроса REST-биллинга: сеть, статус не 2xx, снимок не прошёл проверку (штуки).",
		}),
		pollsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "churn_billing_polls_total",
			Help: "Опросы REST-биллинга; result: changed — записан новый снимок, same — без изменений, error — сбой.",
		}, []string{"result"}),
	}
}

// Collectors — метрики адаптера для /metrics.
func (a *Adapter) Collectors() []prometheus.Collector {
	return []prometheus.Collector{a.errorsTotal, a.pollsTotal}
}

// Status — текущее состояние опроса.
func (a *Adapter) Status() PollStatus {
	a.mu.Lock()
	defer a.mu.Unlock()
	return PollStatus{
		Paused: a.paused, Interval: a.cfg.Interval, LastOKAt: a.lastOK, LastError: a.lastErr,
		ErrorsTotal: a.errCount,
	}
}

// Pause ставит опрос на паузу: ручная загрузка главнее.
func (a *Adapter) Pause() {
	a.mu.Lock()
	a.paused = true
	a.mu.Unlock()
}

// Resume включает опрос. Хеш последнего тела забывается: данные в базе могли быть заменены
// вручную, поэтому ближайший опрос запишет снимок биллинга, даже если он не менялся.
func (a *Adapter) Resume() {
	a.mu.Lock()
	a.paused = false
	a.hasHash = false
	a.mu.Unlock()
}

// Run опрашивает биллинг, пока не отменён ctx. Первый опрос идёт сразу; после сбоя пауза растёт
// вдвое с каждым повтором, но не больше maxBackoff.
func (a *Adapter) Run(ctx context.Context) {
	for {
		if !a.Paused() {
			if _, err := a.Poll(ctx); err != nil {
				log.Printf("source: опрос биллинга: %v", err)
			}
		}
		t := time.NewTimer(a.nextDelay())
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

// Paused — опрос на паузе.
func (a *Adapter) Paused() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.paused
}

func (a *Adapter) nextDelay() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	d := a.cfg.Interval
	for i := 0; i < a.fails && d < maxBackoff; i++ {
		d *= 2
	}
	if d > maxBackoff && a.cfg.Interval < maxBackoff {
		d = maxBackoff
	}
	return d
}

// Poll делает один опрос. changed=true, если записан новый снимок. Любой сбой оставляет прежние
// данные нетронутыми и считается в метрике ошибок.
func (a *Adapter) Poll(ctx context.Context) (changed bool, err error) {
	changed, err = a.poll(ctx)
	a.mu.Lock()
	defer a.mu.Unlock()
	switch {
	case err != nil:
		a.fails++
		a.lastErr = err.Error()
		a.errCount++
		a.errorsTotal.Inc()
		a.pollsTotal.WithLabelValues("error").Inc()
	default:
		a.fails = 0
		a.lastErr = ""
		now := a.clk.Now()
		a.lastOK = &now
		if changed {
			a.pollsTotal.WithLabelValues("changed").Inc()
		} else {
			a.pollsTotal.WithLabelValues("same").Inc()
		}
	}
	return changed, err
}

func (a *Adapter) poll(ctx context.Context) (bool, error) {
	body, err := a.fetch(ctx)
	if err != nil {
		return false, err
	}
	sum := sha256.Sum256(body)
	a.mu.Lock()
	same := a.hasHash && a.lastHash == sum
	a.mu.Unlock()
	if same {
		return false, nil
	}
	s, errs, err := ParseSnapshot(bytes.NewReader(body))
	if err != nil {
		return false, fmt.Errorf("снимок не разобран: %w", err)
	}
	errs = append(errs, Validate(s)...)
	if len(errs) > 0 {
		return false, fmt.Errorf("снимок не прошёл проверку: %s: %s", errs[0].Field, errs[0].Message)
	}

	emulatorMu.Lock() // не пересекаться с шагами эмулятора и правками
	defer emulatorMu.Unlock()
	// Ручная загрузка могла встать на паузу, пока шёл запрос: не затираем её.
	if a.Paused() {
		return false, nil
	}
	now := a.clk.Now()
	acc, err := replace(ctx, a.pool, s, ModelTime(s, now), now, OriginAPI)
	if err != nil {
		return false, fmt.Errorf("снимок не записан, прежние данные сохранены: %w", err)
	}
	a.mu.Lock()
	a.lastHash, a.hasHash = sum, true
	a.mu.Unlock()
	a.bus.Publish(TopicChanged, acc.Version)
	return true, nil
}

func (a *Adapter) fetch(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.cfg.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("адрес биллинга: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if a.cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+a.cfg.Token)
	}
	resp, err := a.http.Do(req)
	if err != nil {
		// В url.Error адрес вместе с возможным токеном в строке запроса: оставляем причину.
		if inner := errors.Unwrap(err); inner != nil {
			err = inner
		}
		return nil, fmt.Errorf("биллинг недоступен: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("биллинг ответил %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, a.maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("тело ответа не прочитано: %w", err)
	}
	if int64(len(body)) > a.maxBytes {
		return nil, fmt.Errorf("ответ биллинга больше %d байт", a.maxBytes)
	}
	return body, nil
}

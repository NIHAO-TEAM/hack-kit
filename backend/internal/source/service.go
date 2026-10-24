package source

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
)

// Источники снимка в snapshots.source.
const (
	OriginSeed     = "seed"
	OriginAPI      = "api"
	OriginEmulator = "emulator"
)

// TopicChanged — тема шины, в которую после коммита снимка уходит номер версии.
const TopicChanged = "source.changed"

// lockKey — ключ advisory-блокировки: два снимка одновременно не пишутся.
const lockKey = 7301001

// Status — состояние текущего снимка.
type Status struct {
	Version  int64
	AsOf     *time.Time
	LoadedAt time.Time
	Source   string
}

// Accepted — итог записи снимка.
type Accepted struct {
	Version      int64
	AsOf         time.Time
	Accounts     int
	Services     int
	Transactions int
}

// replace заменяет данные целиком в одной транзакции и заводит строку snapshots.
// Любая ошибка откатывает всё: прежние данные и версия остаются.
func replace(ctx context.Context, pool *pgxpool.Pool, s *Snapshot, asOf, loadedAt time.Time, source string) (Accepted, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Accepted{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); err != nil {
		return Accepted{}, err
	}
	if _, err := tx.Exec(ctx, "TRUNCATE transactions, services, accounts"); err != nil {
		return Accepted{}, fmt.Errorf("очистка: %w", err)
	}

	accRows := make([][]any, len(s.Accounts))
	for i, a := range s.Accounts {
		accRows[i] = []any{a.ID, a.Email, a.Balance, a.WithdrawBalance, a.TotalReplenished, a.BonusBalance, a.BonusState}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"accounts"},
		[]string{"id", "email", "balance", "withdrawBalance", "totalReplenished", "bonusBalance", "bonusState"},
		pgx.CopyFromRows(accRows)); err != nil {
		return Accepted{}, fmt.Errorf("accounts: %w", err)
	}

	svcRows := make([][]any, len(s.Services))
	for i, v := range s.Services {
		svcRows[i] = []any{v.ID, v.Name, v.OwnerID, v.PaymentTerm, v.AutoProlong, v.CreatedAt, v.ExpiresAt, v.Status, v.Price}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"services"},
		[]string{"id", "name", "ownerId", "paymentTerm", "autoProlong", "createdAt", "expiresAt", "status", "price"},
		pgx.CopyFromRows(svcRows)); err != nil {
		return Accepted{}, fmt.Errorf("services: %w", err)
	}

	txRows := make([][]any, len(s.Transactions))
	for i, t := range s.Transactions {
		txRows[i] = []any{t.ID, t.Amount, t.Status, t.PerformedAt, t.CreatedAt, t.Type, t.ServiceID}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"transactions"},
		[]string{"id", "amount", "status", "performedAt", "createdAt", "type", "serviceId"},
		pgx.CopyFromRows(txRows)); err != nil {
		return Accepted{}, fmt.Errorf("transactions: %w", err)
	}

	// Идентификаторы заданы снимком; последовательности подтягиваем, чтобы последующие
	// вставки (правки, эмулятор) не упёрлись в занятые id.
	for _, t := range []string{"accounts", "services", "transactions"} {
		q := fmt.Sprintf(`SELECT setval(pg_get_serial_sequence('%[1]s', 'id'), COALESCE(MAX(id), 1), MAX(id) IS NOT NULL) FROM %[1]s`, t)
		if _, err := tx.Exec(ctx, q); err != nil {
			return Accepted{}, fmt.Errorf("sequence %s: %w", t, err)
		}
	}

	var version int64
	if err := tx.QueryRow(ctx,
		"INSERT INTO snapshots (as_of, loaded_at, source) VALUES ($1, $2, $3) RETURNING version",
		asOf, loadedAt, source).Scan(&version); err != nil {
		return Accepted{}, fmt.Errorf("snapshots: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Accepted{}, err
	}
	return Accepted{Version: version, AsOf: asOf, Accounts: len(s.Accounts), Services: len(s.Services), Transactions: len(s.Transactions)}, nil
}

// currentStatus читает последнюю версию снимка; ok=false, если снимков ещё нет.
func currentStatus(ctx context.Context, pool *pgxpool.Pool) (st Status, ok bool, err error) {
	err = pool.QueryRow(ctx,
		"SELECT version, as_of, loaded_at, source FROM snapshots ORDER BY version DESC LIMIT 1").
		Scan(&st.Version, &st.AsOf, &st.LoadedAt, &st.Source)
	if errors.Is(err, pgx.ErrNoRows) {
		return Status{}, false, nil
	}
	return st, err == nil, err
}

// organizers — набор организаторов, копия cases/analizator/data.json.
//
//go:embed seed/organizers.json
var organizers []byte

// seedAsOf — время модели сида: слайд кейса считается на 19.10.2026 15:00 UTC.
var seedAsOf = time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC)

// OnStart загружает набор организаторов, если включён SEED_ORGANIZERS и снимков ещё нет.
// Повторный старт снимок и версию не меняет.
func OnStart(ctx context.Context, pool *pgxpool.Pool, bus *events.Bus, clk clock.Clock, enabled bool) error {
	if !enabled {
		return nil
	}
	if _, ok, err := currentStatus(ctx, pool); err != nil || ok {
		return err
	}
	body, err := seedBody()
	if err != nil {
		return err
	}
	s, errs, err := ParseSnapshot(bytes.NewReader(body))
	if err != nil || len(errs) > 0 {
		return fmt.Errorf("source: вшитый сид не разбирается: %v %v", err, errs)
	}
	acc, err := replace(ctx, pool, s, seedAsOf, clk.Now(), OriginSeed)
	if err != nil {
		return fmt.Errorf("source: сид: %w", err)
	}
	bus.Publish(TopicChanged, acc.Version)
	return nil
}

// ErrNoSnapshot — снимков ещё не загружали.
var ErrNoSnapshot = errors.New("source: снимок ещё не загружен")

// Reader читает текущий снимок из таблиц организаторов.
type Reader struct {
	pool *pgxpool.Pool
}

// NewReader создаёт читателя снимка.
func NewReader(pool *pgxpool.Pool) *Reader { return &Reader{pool: pool} }

// Version читает номер последней версии снимка одним запросом; ErrNoSnapshot, пока снимков нет.
func (r *Reader) Version(ctx context.Context) (int64, error) {
	var version int64
	err := r.pool.QueryRow(ctx, "SELECT version FROM snapshots ORDER BY version DESC LIMIT 1").Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNoSnapshot
	}
	if err != nil {
		return 0, fmt.Errorf("source: версия снимка: %w", err)
	}
	return version, nil
}

// Load читает последнюю строку snapshots и три таблицы в одной транзакции REPEATABLE READ:
// данные и версия согласованы, даже если во время чтения пишется новый снимок.
// AsOf берётся из строки snapshots; пустой as_of (источник не назвал время) — loaded_at.
func (r *Reader) Load(ctx context.Context) (Snapshot, int64, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return Snapshot{}, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		version  int64
		asOf     *time.Time
		loadedAt time.Time
	)
	err = tx.QueryRow(ctx, "SELECT version, as_of, loaded_at FROM snapshots ORDER BY version DESC LIMIT 1").
		Scan(&version, &asOf, &loadedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Snapshot{}, 0, ErrNoSnapshot
	}
	if err != nil {
		return Snapshot{}, 0, fmt.Errorf("source: версия снимка: %w", err)
	}
	at := loadedAt.UTC()
	if asOf != nil {
		at = asOf.UTC()
	}
	snap := Snapshot{AsOf: &at}

	rows, err := tx.Query(ctx, `SELECT id, email, balance, "withdrawBalance", "totalReplenished", "bonusBalance", "bonusState"
		FROM accounts ORDER BY id`)
	if err != nil {
		return Snapshot{}, 0, fmt.Errorf("source: accounts: %w", err)
	}
	snap.Accounts, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (a Account, err error) {
		err = row.Scan(&a.ID, &a.Email, &a.Balance, &a.WithdrawBalance, &a.TotalReplenished, &a.BonusBalance, &a.BonusState)
		return a, err
	})
	if err != nil {
		return Snapshot{}, 0, fmt.Errorf("source: accounts: %w", err)
	}

	rows, err = tx.Query(ctx, `SELECT id, name, "ownerId", "paymentTerm", "autoProlong", "createdAt", "expiresAt", status, price
		FROM services ORDER BY id`)
	if err != nil {
		return Snapshot{}, 0, fmt.Errorf("source: services: %w", err)
	}
	snap.Services, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (v Service, err error) {
		err = row.Scan(&v.ID, &v.Name, &v.OwnerID, &v.PaymentTerm, &v.AutoProlong, &v.CreatedAt, &v.ExpiresAt, &v.Status, &v.Price)
		return v, err
	})
	if err != nil {
		return Snapshot{}, 0, fmt.Errorf("source: services: %w", err)
	}

	rows, err = tx.Query(ctx, `SELECT id, amount, status, "performedAt", "createdAt", type, "serviceId"
		FROM transactions ORDER BY id`)
	if err != nil {
		return Snapshot{}, 0, fmt.Errorf("source: transactions: %w", err)
	}
	snap.Transactions, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (t Transaction, err error) {
		err = row.Scan(&t.ID, &t.Amount, &t.Status, &t.PerformedAt, &t.CreatedAt, &t.Type, &t.ServiceID)
		return t, err
	})
	if err != nil {
		return Snapshot{}, 0, fmt.Errorf("source: transactions: %w", err)
	}
	return snap, version, nil
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
	return backoff(a.cfg.Interval, a.fails)
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

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
	"sync/atomic"
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
	var st stages
	defer st.log()
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Accepted{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := st.run(stageLock, func() error {
		_, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey)
		return err
	}); err != nil {
		return Accepted{}, err
	}
	// Порядок таблиц как у Reader.Load (accounts, services, transactions): иначе TRUNCATE и читатель
	// пересчёта берут блокировки навстречу друг другу и PostgreSQL рвёт одного как deadlock.
	if err := st.run(stageTruncate, func() error {
		_, err := tx.Exec(ctx, "TRUNCATE accounts, services, transactions")
		return err
	}); err != nil {
		return Accepted{}, fmt.Errorf("очистка: %w", err)
	}

	if err := st.run(stageCopyAccounts, func() error {
		_, err := tx.CopyFrom(ctx, pgx.Identifier{"accounts"},
			accountCols,
			pgx.CopyFromSlice(len(s.Accounts), func(i int) ([]any, error) {
				a := s.Accounts[i]
				return []any{a.ID, a.Email, a.Balance, a.WithdrawBalance, a.TotalReplenished, a.BonusBalance, a.BonusState}, nil
			}))
		return err
	}); err != nil {
		return Accepted{}, fmt.Errorf("accounts: %w", err)
	}

	if err := st.run(stageCopyServices, func() error {
		_, err := tx.CopyFrom(ctx, pgx.Identifier{"services"},
			serviceCols,
			pgx.CopyFromSlice(len(s.Services), func(i int) ([]any, error) {
				v := s.Services[i]
				return []any{v.ID, v.Name, v.OwnerID, v.PaymentTerm, v.AutoProlong, v.CreatedAt, v.ExpiresAt, v.Status, v.Price}, nil
			}))
		return err
	}); err != nil {
		return Accepted{}, fmt.Errorf("services: %w", err)
	}

	if err := st.run(stageCopyTransactions, func() error {
		_, err := tx.CopyFrom(ctx, pgx.Identifier{"transactions"},
			transactionCols,
			pgx.CopyFromSlice(len(s.Transactions), func(i int) ([]any, error) {
				x := s.Transactions[i]
				return []any{x.ID, x.Amount, x.Status, x.PerformedAt, x.CreatedAt, x.Type, x.ServiceID}, nil
			}))
		return err
	}); err != nil {
		return Accepted{}, fmt.Errorf("transactions: %w", err)
	}

	// Идентификаторы заданы снимком; последовательности подтягиваем, чтобы последующие
	// вставки (правки, эмулятор) не упёрлись в занятые id.
	if err := st.run(stageSequences, func() error { return setSequences(ctx, tx) }); err != nil {
		return Accepted{}, err
	}

	var version int64
	if err := tx.QueryRow(ctx,
		"INSERT INTO snapshots (as_of, loaded_at, source) VALUES ($1, $2, $3) RETURNING version",
		asOf, loadedAt, source).Scan(&version); err != nil {
		return Accepted{}, fmt.Errorf("snapshots: %w", err)
	}
	if err := st.run(stageCommit, func() error { return tx.Commit(ctx) }); err != nil {
		return Accepted{}, err
	}
	return Accepted{Version: version, AsOf: asOf, Accounts: len(s.Accounts), Services: len(s.Services), Transactions: len(s.Transactions)}, nil
}

// Столбцы COPY трёх таблиц организаторов.
var (
	accountCols     = []string{"id", "email", "balance", "withdrawBalance", "totalReplenished", "bonusBalance", "bonusState"}
	serviceCols     = []string{"id", "name", "ownerId", "paymentTerm", "autoProlong", "createdAt", "expiresAt", "status", "price"}
	transactionCols = []string{"id", "amount", "status", "performedAt", "createdAt", "type", "serviceId"}
)

// setSequences подтягивает последовательности: идентификаторы заданы снимком, а последующие
// вставки (правки, эмулятор) не должны упереться в занятые id.
func setSequences(ctx context.Context, tx pgx.Tx) error {
	for _, t := range []string{"accounts", "services", "transactions"} {
		q := fmt.Sprintf(`SELECT setval(pg_get_serial_sequence('%[1]s', 'id'), COALESCE(MAX(id), 1), MAX(id) IS NOT NULL) FROM %[1]s`, t)
		if _, err := tx.Exec(ctx, q); err != nil {
			return fmt.Errorf("sequence %s: %w", t, err)
		}
	}
	return nil
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

// ingester ведёт одну загрузку: разбор → проверка → запись COPY в транзакции.
type ingester struct {
	ctx    context.Context
	cancel context.CancelFunc
	pool   *pgxpool.Pool
	lock   sync.Locker
	locked bool
	st     stages
	tx     pgx.Tx
	in     <-chan item // после проверки
	bad    atomic.Bool // проверка нашла ошибку: писать дальше незачем, но собрать остальные ошибки надо

	discard bool  // запись прекращена: ошибка проверки или базы; поток дочитывается впустую
	dbErr   error // первая ошибка базы
	copied  [3]bool
	early   struct { // массивы, пришедшие раньше тех, на которые ссылаются внешние ключи
		services     []Service
		transactions []Transaction
	}
	counts [3]int
}

func (w *ingester) beginTx() error {
	if w.tx != nil {
		return nil
	}
	w.lock.Lock() // не пересекаться с опросом биллинга, эмулятором и правками
	w.locked = true
	tx, err := w.pool.Begin(w.ctx)
	if err != nil {
		return err
	}
	w.tx = tx
	if err := w.st.run(stageLock, func() error {
		_, err := tx.Exec(w.ctx, "SELECT pg_advisory_xact_lock($1)", lockKey)
		return err
	}); err != nil {
		return err
	}
	// Порядок таблиц как у Reader.Load, см. replace.
	if err := w.st.run(stageTruncate, func() error {
		_, err := tx.Exec(w.ctx, "TRUNCATE accounts, services, transactions")
		return err
	}); err != nil {
		return fmt.Errorf("очистка: %w", err)
	}
	return nil
}

// fail запоминает первую ошибку базы и останавливает разбор и проверку.
func (w *ingester) fail(err error) {
	if w.dbErr == nil {
		w.dbErr = err
	}
	w.discard = true
	w.cancel()
}

// skipArray дочитывает текущий массив впустую.
func (w *ingester) skipArray() {
	for it := range w.in {
		if it.kind == itemEnd {
			return
		}
	}
}

var copyStage = [3]string{stageCopyAccounts, stageCopyServices, stageCopyTransactions}

// table забирает из канала один массив целиком: пишет его COPY, а если таблица, на которую
// ссылается внешний ключ, ещё не записана, откладывает до её конца.
func (w *ingester) table(t table) {
	if w.discard {
		w.skipArray()
		return
	}
	if err := w.beginTx(); err != nil {
		w.fail(err)
		w.skipArray()
		return
	}
	if dep, ok := dependsOn(t); ok && !w.copied[dep] {
		w.hold(t)
		return
	}
	src := &chanSource{w: w, tbl: t}
	err := w.st.run(copyStage[t], func() error {
		_, err := w.tx.CopyFrom(w.ctx, pgx.Identifier{t.String()}, columns[t], src)
		return err
	})
	switch {
	case src.aborted:
		w.discard = true // проверка нашла ошибку, транзакция откатится
		w.skipArray()
		return
	case err != nil:
		w.fail(fmt.Errorf("%s: %w", t, err))
		w.skipArray()
		return
	}
	w.counts[t] += src.n
	w.written(t)
}

// hold откладывает массив в память до записи того, от чего зависит его внешний ключ.
// Тело в «естественном» порядке (accounts, services, transactions) до сюда не доходит.
func (w *ingester) hold(t table) {
	for it := range w.in {
		if it.kind == itemEnd {
			return
		}
		switch t {
		case tableServices:
			w.early.services = append(w.early.services, it.svc)
		case tableTransactions:
			w.early.transactions = append(w.early.transactions, it.tx)
		}
	}
}

// written отмечает таблицу записанной и дописывает отложенные массивы, которые ждали её.
func (w *ingester) written(t table) {
	w.copied[t] = true
	var err error
	switch {
	case t == tableAccounts && len(w.early.services) > 0:
		svcs := w.early.services
		w.early.services = nil
		err = w.st.run(stageCopyServices, func() error {
			_, err := w.tx.CopyFrom(w.ctx, pgx.Identifier{"services"}, serviceCols,
				pgx.CopyFromSlice(len(svcs), func(i int) ([]any, error) { return serviceRow(&svcs[i]), nil }))
			return err
		})
		if err == nil {
			w.counts[tableServices] += len(svcs)
			w.written(tableServices)
		}
	case t == tableServices && len(w.early.transactions) > 0:
		txs := w.early.transactions
		w.early.transactions = nil
		err = w.st.run(stageCopyTransactions, func() error {
			_, err := w.tx.CopyFrom(w.ctx, pgx.Identifier{"transactions"}, transactionCols,
				pgx.CopyFromSlice(len(txs), func(i int) ([]any, error) { return transactionRow(&txs[i]), nil }))
			return err
		})
		if err == nil {
			w.counts[tableTransactions] += len(txs)
		}
	}
	if err != nil {
		w.fail(err)
	}
}

var columns = [3][]string{accountCols, serviceCols, transactionCols}

func accountRow(a *Account) []any {
	return []any{a.ID, a.Email, a.Balance, a.WithdrawBalance, a.TotalReplenished, a.BonusBalance, a.BonusState}
}

func serviceRow(v *Service) []any {
	return []any{v.ID, v.Name, v.OwnerID, v.PaymentTerm, v.AutoProlong, v.CreatedAt, v.ExpiresAt, v.Status, v.Price}
}

func transactionRow(x *Transaction) []any {
	return []any{x.ID, x.Amount, x.Status, x.PerformedAt, x.CreatedAt, x.Type, x.ServiceID}
}

// chanSource отдаёт COPY записи одного массива прямо из канала, до метки конца массива.
type chanSource struct {
	w       *ingester
	tbl     table
	cur     []any
	n       int
	aborted bool
}

func (s *chanSource) Next() bool {
	if s.w.bad.Load() {
		s.aborted = true
		return false
	}
	it, ok := <-s.w.in
	if !ok || it.kind == itemEnd {
		return false
	}
	switch s.tbl {
	case tableAccounts:
		s.cur = accountRow(&it.acc)
	case tableServices:
		s.cur = serviceRow(&it.svc)
	default:
		s.cur = transactionRow(&it.tx)
	}
	s.n++
	return true
}

func (s *chanSource) Values() ([]any, error) { return s.cur, nil }
func (s *chanSource) Err() error {
	if s.aborted {
		return errors.New("загрузка прервана: снимок не прошёл проверку")
	}
	return nil
}

// ingest принимает снимок потоком и заменяет им данные в одной транзакции.
//
// Три горутины на каналах с ограниченными буферами: разбор тела по токенам → проверка записей →
// запись COPY (эта же горутина держит транзакцию). Запись, разобранная и проверенная, сразу уходит в
// базу; пока база занята, буферы заполняются и разбор ждёт. Тело и снимок целиком в памяти не лежат,
// растут только множества id для проверки повторов и ссылок.
//
// Результат: *BodyError — тело не разобрано (400); непустой []FieldError — снимок не прошёл
// проверку (422); иная ошибка — база (500). При любой ошибке транзакция откатывается и прежние данные
// остаются. lock берётся при первой записи (не раньше) и держится до конца вместе с after, которая
// вызывается после коммита.
func ingest(ctx context.Context, pool *pgxpool.Pool, body io.Reader, now time.Time, origin string,
	lock sync.Locker, after func(Accepted)) (Accepted, []FieldError, error) {
	peak := startPeakSampler()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	parsed := make(chan item, pipeBuffer)
	checked := make(chan item, pipeBuffer)
	w := &ingester{ctx: ctx, cancel: cancel, pool: pool, lock: lock, in: checked}
	defer func() {
		if w.tx != nil {
			_ = w.tx.Rollback(context.Background())
		}
		if w.locked {
			lock.Unlock()
		}
	}()

	var (
		asOf     *time.Time
		fieldErr []FieldError
		parseErr error
		ph       = &sendHandler{sender{ctx: ctx, ch: parsed}}
		val      = &validateStage{v: newValidator(0), bad: &w.bad, in: parsed, sender: sender{ctx: ctx, ch: checked}}
		parseDur time.Duration
		wg       sync.WaitGroup
	)
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer close(parsed)
		start := time.Now()
		asOf, fieldErr, parseErr = parseStream(body, ph, maxErrors)
		parseDur = time.Since(start) - ph.blocked
	}()
	go func() {
		defer wg.Done()
		defer close(checked)
		val.run()
	}()

	for it := range checked {
		if it.kind == itemBegin {
			w.table(it.tbl)
		}
	}
	wg.Wait()
	heap := peak.finish()

	w.st.add(stageParse, parseDur)
	w.st.add(stageValidate, val.busy)
	defer func() {
		w.st.log()
		log.Printf("source: куча при загрузке: до %d МБ, пик %d МБ", peak.start>>20, heap>>20)
	}()

	// Приоритет ошибок: тело не разобрано, затем проверка, затем база.
	if parseErr != nil && (w.dbErr == nil || !errors.Is(parseErr, context.Canceled)) {
		return Accepted{}, nil, &BodyError{parseErr}
	}
	errs := append(fieldErr, val.v.finish()...)
	if len(errs) > maxErrors {
		errs = errs[:maxErrors]
	}
	if len(errs) > 0 {
		return Accepted{}, errs, nil
	}
	if w.dbErr != nil {
		return Accepted{}, nil, w.dbErr
	}
	if val.fatal != nil {
		return Accepted{}, nil, val.fatal
	}

	modelAt := val.latest.modelTime(asOf, now)
	if err := w.st.run(stageSequences, func() error { return setSequences(ctx, w.tx) }); err != nil {
		return Accepted{}, nil, err
	}
	var version int64
	if err := w.tx.QueryRow(ctx,
		"INSERT INTO snapshots (as_of, loaded_at, source) VALUES ($1, $2, $3) RETURNING version",
		modelAt, now, origin).Scan(&version); err != nil {
		return Accepted{}, nil, fmt.Errorf("snapshots: %w", err)
	}
	if err := w.st.run(stageCommit, func() error { return w.tx.Commit(ctx) }); err != nil {
		return Accepted{}, nil, err
	}
	w.tx = nil
	acc := Accepted{Version: version, AsOf: modelAt,
		Accounts: w.counts[tableAccounts], Services: w.counts[tableServices], Transactions: w.counts[tableTransactions]}
	if after != nil {
		after(acc)
	}
	return acc, nil, nil
}

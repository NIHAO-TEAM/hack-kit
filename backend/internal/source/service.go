package source

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
)

// Источники снимка в snapshots.source.
const (
	OriginSeed = "seed"
	OriginAPI  = "api"
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

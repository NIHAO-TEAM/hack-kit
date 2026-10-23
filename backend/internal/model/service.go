package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/platform/events"
)

// TopicChanged — тема шины, в которую после коммита настроек уходит номер их версии (int64).
const TopicChanged = "model.changed"

// lockKey — ключ advisory-блокировки: два Save одновременно не пишутся.
const lockKey = 7301002

// Validator проверяет тело настроек до записи. Смысл настроек знает ядро модели (risk),
// а модуль model хранит их как JSON, поэтому проверку подставляет сборка сервера.
type Validator func(body []byte) error

// Store хранит настройки модели в таблице model_settings.
type Store struct {
	pool     *pgxpool.Pool
	bus      *events.Bus
	validate Validator
}

// NewStore создаёт хранилище настроек. validate может быть nil — тогда тело не проверяется.
func NewStore(pool *pgxpool.Pool, bus *events.Bus, validate Validator) *Store {
	return &Store{pool: pool, bus: bus, validate: validate}
}

// Current возвращает тело и номер версии последних сохранённых настроек. Пока ничего
// не сохраняли — nil и версия 0: умолчания подставляет тот, кто знает смысл настроек.
func (s *Store) Current(ctx context.Context) (json.RawMessage, int64, error) {
	var (
		body    []byte
		version int64
	)
	err := s.pool.QueryRow(ctx, "SELECT body, version FROM model_settings ORDER BY version DESC LIMIT 1").
		Scan(&body, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, fmt.Errorf("model: чтение настроек: %w", err)
	}
	return body, version, nil
}

// Save проверяет тело, записывает новую версию и публикует model.changed.
// Неверное тело — ошибка, версия прежняя.
func (s *Store) Save(ctx context.Context, body json.RawMessage, by string) (int64, error) {
	if s.validate != nil {
		if err := s.validate(body); err != nil {
			return 0, fmt.Errorf("model: настройки: %w", err)
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", lockKey); err != nil {
		return 0, err
	}
	var version int64
	if err := tx.QueryRow(ctx,
		"INSERT INTO model_settings (body, updated_by) VALUES ($1, $2) RETURNING version", []byte(body), by).
		Scan(&version); err != nil {
		return 0, fmt.Errorf("model: запись настроек: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	s.bus.Publish(TopicChanged, version)
	return version, nil
}

// Record — сохранённая версия настроек с автором и временем.
type Record struct {
	Body      json.RawMessage
	Version   int64
	UpdatedBy string
	UpdatedAt time.Time
}

// Latest возвращает последнюю версию настроек целиком. Ничего не сохраняли — нулевая запись
// (Body nil, версия 0).
func (s *Store) Latest(ctx context.Context) (Record, error) {
	var rec Record
	var body []byte
	err := s.pool.QueryRow(ctx,
		"SELECT body, version, updated_by, updated_at FROM model_settings ORDER BY version DESC LIMIT 1").
		Scan(&body, &rec.Version, &rec.UpdatedBy, &rec.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Record{}, nil
	}
	if err != nil {
		return Record{}, fmt.Errorf("model: чтение настроек: %w", err)
	}
	rec.Body = body
	return rec, nil
}

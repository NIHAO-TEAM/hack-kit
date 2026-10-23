package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/platform/events"
	"example.com/app/internal/risk"
)

// TopicChanged — тема шины, в которую после коммита настроек уходит номер их версии (int64).
const TopicChanged = "model.changed"

// SettingsReader отдаёт текущие настройки модели и номер их версии.
type SettingsReader interface {
	Current(ctx context.Context) (risk.Settings, int64, error)
}

// Store хранит настройки модели в таблице model_settings.
type Store struct {
	pool *pgxpool.Pool
	bus  *events.Bus
}

// NewStore создаёт хранилище настроек.
func NewStore(pool *pgxpool.Pool, bus *events.Bus) *Store {
	return &Store{pool: pool, bus: bus}
}

// lockKey — ключ advisory-блокировки: два Save одновременно не пишутся.
const lockKey = 7301002

// Current возвращает последнюю сохранённую версию настроек; пока таблица пуста —
// risk.Defaults() с версией 0.
func (s *Store) Current(ctx context.Context) (risk.Settings, int64, error) {
	var (
		body    []byte
		version int64
	)
	err := s.pool.QueryRow(ctx, "SELECT body, version FROM model_settings ORDER BY version DESC LIMIT 1").
		Scan(&body, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return risk.Defaults(), 0, nil
	}
	if err != nil {
		return risk.Settings{}, 0, fmt.Errorf("model: чтение настроек: %w", err)
	}
	var st risk.Settings
	if err := json.Unmarshal(body, &st); err != nil {
		return risk.Settings{}, 0, fmt.Errorf("model: разбор настроек версии %d: %w", version, err)
	}
	return st, version, nil
}

// Save проверяет настройки, записывает новую версию и публикует model.changed.
// Неверные настройки — ошибка, версия прежняя.
func (s *Store) Save(ctx context.Context, st risk.Settings, by string) (int64, error) {
	if err := st.Validate(); err != nil {
		return 0, fmt.Errorf("model: настройки: %w", err)
	}
	body, err := json.Marshal(st)
	if err != nil {
		return 0, err
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
		"INSERT INTO model_settings (body, updated_by) VALUES ($1, $2) RETURNING version", body, by).
		Scan(&version); err != nil {
		return 0, fmt.Errorf("model: запись настроек: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	s.bus.Publish(TopicChanged, version)
	return version, nil
}

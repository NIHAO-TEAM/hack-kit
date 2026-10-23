package audit

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/platform/clock"
)

// Service пишет и читает таблицу audit_log.
type Service struct {
	pool  *pgxpool.Pool
	clk   clock.Clock
	actor func(context.Context) string
}

// tokenActor — автор записи, когда действует служебный токен, а не вошедший пользователь.
const tokenActor = "служебный токен"

// WithActor задаёт, как узнать логин по контексту запроса; Record подставляет его в записи
// без автора. Так модули не знают друг о друге.
func (s *Service) WithActor(f func(context.Context) string) *Service {
	s.actor = f
	return s
}

// New создаёт журнал. pool nil (тесты без базы) — записи отбрасываются.
func New(pool *pgxpool.Pool, clk clock.Clock) *Service {
	return &Service{pool: pool, clk: clk}
}

// Record добавляет запись. Ошибка записи не ломает действие пользователя: её видно в логе.
func (s *Service) Record(ctx context.Context, e Entry) {
	if s == nil || s.pool == nil {
		return
	}
	if e.Actor == "" && s.actor != nil {
		e.Actor = s.actor(ctx)
		if e.Actor == "" {
			e.Actor = tokenActor
		}
	}
	if e.Changes == nil {
		e.Changes = []Change{}
	}
	changes, err := json.Marshal(e.Changes)
	if err != nil {
		logf("кодирование: %v", err)
		return
	}
	at := e.At
	if at.IsZero() {
		at = s.clk.Now()
	}
	if _, err := s.pool.Exec(ctx,
		"INSERT INTO audit_log (at, actor, action, summary, changes) VALUES ($1, $2, $3, $4, $5)",
		at, e.Actor, e.Action, e.Summary, changes); err != nil {
		logf("запись: %v", err)
	}
}

// List возвращает страницу журнала, новые сверху, и общее число записей.
func (s *Service) List(ctx context.Context, limit, offset int) ([]Entry, int, error) {
	var total int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM audit_log").Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("audit: подсчёт: %w", err)
	}
	rows, err := s.pool.Query(ctx,
		"SELECT id, at, actor, action, summary, changes FROM audit_log ORDER BY at DESC, id DESC LIMIT $1 OFFSET $2",
		limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("audit: чтение: %w", err)
	}
	defer rows.Close()
	items := []Entry{}
	for rows.Next() {
		var e Entry
		var raw []byte
		if err := rows.Scan(&e.ID, &e.At, &e.Actor, &e.Action, &e.Summary, &raw); err != nil {
			return nil, 0, fmt.Errorf("audit: чтение: %w", err)
		}
		if err := json.Unmarshal(raw, &e.Changes); err != nil {
			return nil, 0, fmt.Errorf("audit: изменения записи %d: %w", e.ID, err)
		}
		items = append(items, e)
	}
	return items, total, rows.Err()
}

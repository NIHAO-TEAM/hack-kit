package retention

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/analytics"
	"example.com/app/internal/audit"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
)

// Ошибки команд; HTTP-слой переводит их в коды ответа.
var (
	ErrNotFound         = errors.New("retention: задача не найдена")
	ErrCustomerNotFound = errors.New("retention: клиента нет в текущем снимке")
	ErrNotReady         = errors.New("retention: расчёт ещё не готов")
	ErrClosed           = errors.New("retention: задача закрыта")
)

// ConflictError — конфликт с чужой правкой: Task — текущее состояние, By — кто изменил последним.
type ConflictError struct {
	Task gen.RetentionTask
	By   string
	// Duplicate — у клиента уже есть открытая задача (а не устаревшая версия).
	Duplicate bool
}

func (e *ConflictError) Error() string {
	return "retention: конфликт задачи " + strconv.FormatInt(e.Task.Id, 10)
}

// ValidationError — запрос не прошёл проверку; Fields — по полям.
type ValidationError struct{ Fields []FieldError }

func (e *ValidationError) Error() string {
	return "retention: запрос не прошёл проверку"
}

// Service хранит задачи в PostgreSQL.
type Service struct {
	pool  *pgxpool.Pool
	clk   clock.Clock
	bus   *events.Bus
	feed  analytics.Feed
	rec   audit.Recorder
	actor func(context.Context) string
}

// New создаёт сервис. pool nil (тесты без базы) — обработчики отвечают 501.
func New(pool *pgxpool.Pool, bus *events.Bus, clk clock.Clock, feed analytics.Feed) *Service {
	return &Service{pool: pool, bus: bus, clk: clk, feed: feed}
}

// WithAudit подключает журнал действий.
func (s *Service) WithAudit(r audit.Recorder) *Service {
	s.rec = r
	return s
}

// WithActor задаёт, как узнать логин по контексту запроса.
func (s *Service) WithActor(f func(context.Context) string) *Service {
	s.actor = f
	return s
}

func (s *Service) login(ctx context.Context) string {
	if s.actor == nil {
		return ""
	}
	return s.actor(ctx)
}

const taskColumns = `id, customer_id, customer_email, status, outcome, assignee, due_at, note, version,
	baseline, created_by, created_at, updated_by, updated_at, closed_at,
	(SELECT e.text FROM retention_task_events e WHERE e.task_id = retention_tasks.id AND e.kind = 'observed' ORDER BY e.id DESC LIMIT 1)`

type scanner interface{ Scan(dest ...any) error }

func scanTask(row scanner) (gen.RetentionTask, error) {
	var t gen.RetentionTask
	var email string
	var outcome *string
	var status string
	var baseline []byte
	if err := row.Scan(&t.Id, &t.CustomerId, &email, &status, &outcome, &t.Assignee, &t.DueAt, &t.Note, &t.Version,
		&baseline, &t.CreatedBy, &t.CreatedAt, &t.UpdatedBy, &t.UpdatedAt, &t.ClosedAt, &t.LastObservation); err != nil {
		return t, err
	}
	t.CustomerEmail = &email
	t.Status = gen.TaskStatus(status)
	if outcome != nil {
		o := gen.TaskOutcome(*outcome)
		t.Outcome = &o
	}
	if err := json.Unmarshal(baseline, &t.Baseline); err != nil {
		return t, fmt.Errorf("retention: исходное состояние задачи %d: %w", t.Id, err)
	}
	return t, nil
}

// decorate дописывает вычисляемые поля: просрочена ли задача и есть ли клиент в текущем снимке.
func (s *Service) decorate(t *gen.RetentionTask) {
	now := s.clk.Now()
	t.Overdue = overdue(t.Status, t.DueAt, now)
	t.CustomerMissing = false
	if s.feed == nil {
		return
	}
	pf := s.feed.Portfolio()
	if pf == nil {
		return
	}
	c, ok := findCustomer(pf, t.CustomerId)
	t.CustomerMissing = !ok || t.CustomerEmail == nil || !strings.EqualFold(strings.TrimSpace(c.Row.Email), strings.TrimSpace(*t.CustomerEmail))
}

func findCustomer(pf *analytics.Portfolio, id int64) (analytics.Customer, bool) {
	i, found := slices.BinarySearchFunc(pf.Customers, id, func(c analytics.Customer, id int64) int {
		return cmp.Compare(c.Row.Id, id)
	})
	if !found {
		return analytics.Customer{}, false
	}
	return pf.Customers[i], true
}

type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func loadEvents(ctx context.Context, q queryer, taskID int64) ([]gen.TaskEvent, error) {
	rows, err := q.Query(ctx,
		`SELECT id, at, actor, kind, channel, text, changes FROM retention_task_events WHERE task_id = $1 ORDER BY id`, taskID)
	if err != nil {
		return nil, fmt.Errorf("retention: история: %w", err)
	}
	defer rows.Close()
	out := []gen.TaskEvent{}
	for rows.Next() {
		var e gen.TaskEvent
		var kind string
		var channel *string
		var raw []byte
		if err := rows.Scan(&e.Id, &e.At, &e.Actor, &kind, &channel, &e.Text, &raw); err != nil {
			return nil, fmt.Errorf("retention: история: %w", err)
		}
		e.Kind = gen.TaskEventKind(kind)
		if channel != nil {
			c := gen.TaskChannel(*channel)
			e.Channel = &c
		}
		e.Changes = []gen.AuditChange{}
		if err := json.Unmarshal(raw, &e.Changes); err != nil {
			return nil, fmt.Errorf("retention: изменения события %d: %w", e.Id, err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Get возвращает задачу с историей.
func (s *Service) Get(ctx context.Context, id int64) (gen.RetentionTask, error) {
	return s.get(ctx, s.pool, id)
}

func (s *Service) get(ctx context.Context, q queryer, id int64) (gen.RetentionTask, error) {
	t, err := scanTask(q.QueryRow(ctx, "SELECT "+taskColumns+" FROM retention_tasks WHERE id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	if err != nil {
		return t, fmt.Errorf("retention: чтение задачи: %w", err)
	}
	ev, err := loadEvents(ctx, q, id)
	if err != nil {
		return t, err
	}
	t.Events = &ev
	s.decorate(&t)
	return t, nil
}

// ListFilter — фильтры списка задач.
type ListFilter struct {
	Status   *gen.TaskStatus
	Assignee string
	Overdue  bool
	Limit    int
	Offset   int
}

// List возвращает страницу задач без истории, свежие правки сверху, и общее число по фильтру.
func (s *Service) List(ctx context.Context, f ListFilter) ([]gen.RetentionTask, int, error) {
	where, args := []string{"true"}, []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.Replace(cond, "?", "$"+strconv.Itoa(len(args)), 1))
	}
	if f.Status != nil {
		add("status = ?", string(*f.Status))
	}
	if f.Assignee != "" {
		add("assignee = ?", f.Assignee)
	}
	if f.Overdue {
		add("status <> 'closed' AND due_at < ?", s.clk.Now())
	}
	cond := strings.Join(where, " AND ")
	var total int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM retention_tasks WHERE "+cond, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("retention: подсчёт задач: %w", err)
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := s.pool.Query(ctx, fmt.Sprintf("SELECT %s FROM retention_tasks WHERE %s ORDER BY updated_at DESC, id DESC LIMIT $%d OFFSET $%d",
		taskColumns, cond, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("retention: список задач: %w", err)
	}
	defer rows.Close()
	items := []gen.RetentionTask{}
	for rows.Next() {
		t, err := scanTask(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("retention: список задач: %w", err)
		}
		s.decorate(&t)
		items = append(items, t)
	}
	return items, total, rows.Err()
}

// Assignees — сотрудники, которым можно назначить задачу.
func (s *Service) Assignees(ctx context.Context) ([]gen.TaskAssignee, error) {
	rows, err := s.pool.Query(ctx, "SELECT login, role FROM users WHERE role IN ('marketer', 'jury', 'admin') ORDER BY login")
	if err != nil {
		return nil, fmt.Errorf("retention: сотрудники: %w", err)
	}
	defer rows.Close()
	out := []gen.TaskAssignee{}
	for rows.Next() {
		var a gen.TaskAssignee
		var role string
		if err := rows.Scan(&a.Login, &role); err != nil {
			return nil, fmt.Errorf("retention: сотрудники: %w", err)
		}
		a.Role = gen.Role(role)
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *Service) canAssign(ctx context.Context, q queryer, login string) (bool, error) {
	var n int
	err := q.QueryRow(ctx, "SELECT count(*) FROM users WHERE login = $1 AND role IN ('marketer', 'jury', 'admin')", login).Scan(&n)
	return n > 0, err
}

// OpenTasks отдаёт открытые задачи по ID клиента для строк очереди и фильтра.
func (s *Service) OpenTasks(ctx context.Context) (map[int64]analytics.TaskRef, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, customer_id, customer_email, status, assignee, due_at
		FROM retention_tasks WHERE status <> 'closed'`)
	if err != nil {
		return nil, fmt.Errorf("retention: открытые задачи: %w", err)
	}
	defer rows.Close()
	now := s.clk.Now()
	out := map[int64]analytics.TaskRef{}
	for rows.Next() {
		var b gen.TaskBrief
		var customerID int64
		var email, status string
		if err := rows.Scan(&b.Id, &customerID, &email, &status, &b.Assignee, &b.DueAt); err != nil {
			return nil, fmt.Errorf("retention: открытые задачи: %w", err)
		}
		b.Status = gen.TaskStatus(status)
		b.Overdue = overdue(b.Status, b.DueAt, now)
		out[customerID] = analytics.TaskRef{Email: email, Brief: b}
	}
	return out, rows.Err()
}

// baseline собирает исходное состояние клиента из текущего результата модели.
func (s *Service) baseline(pf *analytics.Portfolio, c analytics.Customer) gen.TaskBaseline {
	b := gen.TaskBaseline{
		Version:           pf.Version,
		Score:             c.Row.Score,
		Level:             c.Row.Level,
		ExpectedLossCents: c.Row.ExpectedLossCents,
		TopAction:         c.Row.TopAction,
		Balance:           c.Row.Balance,
		Services:          make([]gen.TaskServiceBaseline, 0, len(c.Services)),
	}
	if o, ok := s.feed.(interface{ Overview() *analytics.Overview }); ok {
		if ov := o.Overview(); ov != nil {
			asOf := ov.AsOf
			b.AsOf = &asOf
		}
	}
	for _, v := range c.Services {
		b.Services = append(b.Services, gen.TaskServiceBaseline{Id: v.Id, Status: v.Status, AutoProlong: v.AutoProlong, ExpiresAt: v.ExpiresAt})
	}
	return b
}

// TakeInput — необязательные поля команды «взять в работу».
type TakeInput struct {
	Assignee *string
	DueAt    *time.Time
	Note     *string
}

// Take создаёт задачу по клиенту. Вторая открытая задача на того же клиента — ConflictError с первой.
func (s *Service) Take(ctx context.Context, customerID int64, in TakeInput) (gen.RetentionTask, error) {
	if s.feed == nil || s.feed.Portfolio() == nil {
		return gen.RetentionTask{}, ErrNotReady
	}
	pf := s.feed.Portfolio()
	c, ok := findCustomer(pf, customerID)
	if !ok {
		return gen.RetentionTask{}, ErrCustomerNotFound
	}
	actor := s.login(ctx)
	now := s.clk.Now()
	assignee, due, note := actor, defaultDue(now), ""
	var errs []FieldError
	if in.Assignee != nil {
		assignee = strings.TrimSpace(*in.Assignee)
		if assignee == "" {
			errs = append(errs, FieldError{"assignee", "логин не может быть пустым"})
		}
	}
	if in.DueAt != nil {
		due = in.DueAt.UTC()
		if !due.After(now) {
			errs = append(errs, FieldError{"dueAt", "срок должен быть в будущем"})
		}
	}
	if in.Note != nil {
		note = *in.Note
		if tooLong(note) {
			errs = append(errs, FieldError{"note", "заметка длиннее 2000 символов"})
		}
	}
	if len(errs) > 0 {
		return gen.RetentionTask{}, &ValidationError{errs}
	}
	if ok, err := s.canAssign(ctx, s.pool, assignee); err != nil {
		return gen.RetentionTask{}, fmt.Errorf("retention: проверка ответственного: %w", err)
	} else if !ok {
		return gen.RetentionTask{}, &ValidationError{[]FieldError{{"assignee", "такого сотрудника нет или ему нельзя назначать задачи"}}}
	}
	baseline, err := json.Marshal(s.baseline(pf, c))
	if err != nil {
		return gen.RetentionTask{}, fmt.Errorf("retention: исходное состояние: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return gen.RetentionTask{}, fmt.Errorf("retention: транзакция: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var id int64
	err = tx.QueryRow(ctx, `INSERT INTO retention_tasks
		(customer_id, customer_email, status, assignee, due_at, note, baseline, created_by, created_at, updated_by, updated_at)
		VALUES ($1, $2, 'in_work', $3, $4, $5, $6, $7, $8, $7, $8) RETURNING id`,
		customerID, c.Row.Email, assignee, due, note, baseline, actor, now).Scan(&id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		_ = tx.Rollback(ctx)
		return gen.RetentionTask{}, s.duplicate(ctx, customerID)
	}
	if err != nil {
		return gen.RetentionTask{}, fmt.Errorf("retention: создание задачи: %w", err)
	}
	changes, _ := json.Marshal([]audit.Change{})
	if _, err := tx.Exec(ctx, `INSERT INTO retention_task_events (task_id, at, actor, kind, text, changes)
		VALUES ($1, $2, $3, 'created', $4, $5)`, id, now, actor, "Взят в работу", changes); err != nil {
		return gen.RetentionTask{}, fmt.Errorf("retention: история: %w", err)
	}
	t, err := s.get(ctx, tx, id)
	if err != nil {
		return t, err
	}
	if err := tx.Commit(ctx); err != nil {
		return t, fmt.Errorf("retention: фиксация: %w", err)
	}
	s.recordAudit(ctx, audit.ActionTaskCreated, actor, "Клиент взят в работу: задача "+strconv.FormatInt(id, 10), nil)
	s.publish("created", t)
	return t, nil
}

// duplicate собирает ConflictError с открытой задачей клиента.
func (s *Service) duplicate(ctx context.Context, customerID int64) error {
	var id int64
	err := s.pool.QueryRow(ctx, "SELECT id FROM retention_tasks WHERE customer_id = $1 AND status <> 'closed'", customerID).Scan(&id)
	if err != nil {
		return fmt.Errorf("retention: открытая задача клиента: %w", err)
	}
	t, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	return &ConflictError{Task: t, By: t.UpdatedBy, Duplicate: true}
}

// Patch меняет задачу по версии. Устаревшая версия — ConflictError, данные не меняются.
func (s *Service) Patch(ctx context.Context, id int64, p gen.TaskPatch) (gen.RetentionTask, error) {
	actor := s.login(ctx)
	now := s.clk.Now()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return gen.RetentionTask{}, fmt.Errorf("retention: транзакция: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Блокировка строки: две правки одной версии идут по очереди, вторая видит новую версию.
	cur, err := scanTask(tx.QueryRow(ctx, "SELECT "+taskColumns+" FROM retention_tasks WHERE id = $1 FOR UPDATE", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return cur, ErrNotFound
	}
	if err != nil {
		return cur, fmt.Errorf("retention: чтение задачи: %w", err)
	}
	if cur.Version != p.Version {
		_ = tx.Rollback(ctx)
		return gen.RetentionTask{}, s.stale(ctx, id)
	}
	if cur.Status == gen.Closed {
		return cur, ErrClosed
	}
	next, changes, errs := applyPatch(cur, p, now)
	if len(errs) > 0 {
		return cur, &ValidationError{errs}
	}
	if p.Assignee != nil && next.Assignee != cur.Assignee {
		if ok, err := s.canAssign(ctx, tx, next.Assignee); err != nil {
			return cur, fmt.Errorf("retention: проверка ответственного: %w", err)
		} else if !ok {
			return cur, &ValidationError{[]FieldError{{"assignee", "такого сотрудника нет или ему нельзя назначать задачи"}}}
		}
	}
	if len(changes) == 0 {
		t, err := s.get(ctx, tx, id)
		return t, err
	}
	var outcome *string
	if next.Outcome != nil {
		o := string(*next.Outcome)
		outcome = &o
	}
	if _, err := tx.Exec(ctx, `UPDATE retention_tasks SET status = $2, outcome = $3, assignee = $4, due_at = $5, note = $6,
		version = version + 1, updated_by = $7, updated_at = $8, closed_at = $9 WHERE id = $1`,
		id, string(next.Status), outcome, next.Assignee, next.DueAt, next.Note, actor, now, next.ClosedAt); err != nil {
		return cur, fmt.Errorf("retention: правка задачи: %w", err)
	}
	raw, err := json.Marshal(changes)
	if err != nil {
		return cur, fmt.Errorf("retention: изменения: %w", err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO retention_task_events (task_id, at, actor, kind, text, changes)
		VALUES ($1, $2, $3, 'changed', '', $4)`, id, now, actor, raw); err != nil {
		return cur, fmt.Errorf("retention: история: %w", err)
	}
	t, err := s.get(ctx, tx, id)
	if err != nil {
		return t, err
	}
	if err := tx.Commit(ctx); err != nil {
		return t, fmt.Errorf("retention: фиксация: %w", err)
	}
	s.recordAudit(ctx, audit.ActionTaskChanged, actor, "Изменена задача "+strconv.FormatInt(id, 10), changes)
	s.publish("changed", t)
	return t, nil
}

// stale собирает ConflictError с текущим состоянием задачи.
func (s *Service) stale(ctx context.Context, id int64) error {
	t, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	return &ConflictError{Task: t, By: t.UpdatedBy}
}

// AddEvent записывает контакт или заметку в историю. У закрытой задачи история заморожена.
func (s *Service) AddEvent(ctx context.Context, id int64, in gen.TaskEventCreate) (gen.RetentionTask, error) {
	if errs := validateEvent(in); len(errs) > 0 {
		return gen.RetentionTask{}, &ValidationError{errs}
	}
	actor := s.login(ctx)
	now := s.clk.Now()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return gen.RetentionTask{}, fmt.Errorf("retention: транзакция: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	err = tx.QueryRow(ctx, "SELECT status FROM retention_tasks WHERE id = $1 FOR UPDATE", id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return gen.RetentionTask{}, ErrNotFound
	}
	if err != nil {
		return gen.RetentionTask{}, fmt.Errorf("retention: чтение задачи: %w", err)
	}
	if status == string(gen.Closed) {
		return gen.RetentionTask{}, ErrClosed
	}
	var channel *string
	if in.Channel != nil {
		c := string(*in.Channel)
		channel = &c
	}
	text := ""
	if in.Text != nil {
		text = *in.Text
	}
	if _, err := tx.Exec(ctx, `INSERT INTO retention_task_events (task_id, at, actor, kind, channel, text)
		VALUES ($1, $2, $3, $4, $5, $6)`, id, now, actor, string(in.Kind), channel, text); err != nil {
		return gen.RetentionTask{}, fmt.Errorf("retention: история: %w", err)
	}
	// Контакт — тоже работа с задачей: «связались» обновляет время правки, но не версию.
	if _, err := tx.Exec(ctx, "UPDATE retention_tasks SET updated_at = $2 WHERE id = $1", id, now); err != nil {
		return gen.RetentionTask{}, fmt.Errorf("retention: время правки: %w", err)
	}
	t, err := s.get(ctx, tx, id)
	if err != nil {
		return t, err
	}
	if err := tx.Commit(ctx); err != nil {
		return t, fmt.Errorf("retention: фиксация: %w", err)
	}
	summary := "Заметка в задаче " + strconv.FormatInt(id, 10)
	if in.Kind == gen.TaskEventCreateKindContact {
		summary = "Контакт по задаче " + strconv.FormatInt(id, 10)
	}
	var changes []audit.Change
	if in.Channel != nil {
		changes = []audit.Change{{Field: "channel", From: "", To: string(*in.Channel)}}
	}
	s.recordAudit(ctx, audit.ActionTaskRecorded, actor, summary, changes)
	s.publish("event", t)
	return t, nil
}

func (s *Service) recordAudit(ctx context.Context, action, actor, summary string, changes []audit.Change) {
	if s.rec == nil {
		return
	}
	s.rec.Record(ctx, audit.Entry{Actor: actor, Action: action, Summary: summary, Changes: changes})
}

// publish сообщает открытым экранам, что задача изменилась.
func (s *Service) publish(kind string, t gen.RetentionTask) {
	if s.bus == nil {
		return
	}
	s.bus.Publish(analytics.TopicTasks, analytics.TaskEvent{Type: kind, TaskID: t.Id, CustomerID: t.CustomerId})
}

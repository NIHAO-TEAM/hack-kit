package db_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"example.com/app/internal/platform/db"
	"example.com/app/internal/platform/dbtest"
)

func TestMigrationsUpDownUp(t *testing.T) {
	dsn := dbtest.New(t)
	ctx := context.Background()

	if err := db.Up(ctx, dsn); err != nil {
		t.Fatalf("up: %v", err)
	}
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for _, table := range []string{"accounts", "services", "transactions", "snapshots", "model_settings", "users", "sessions", "retention_tasks", "retention_task_events"} {
		var ok bool
		if err := pool.QueryRow(ctx, "SELECT to_regclass('public.'||$1) IS NOT NULL", table).Scan(&ok); err != nil || !ok {
			t.Errorf("таблицы %s нет после up (err=%v)", table, err)
		}
	}
	// Ограничения организаторов сохранены: баланс не уходит в минус.
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (email, balance) VALUES ('a@example.test', -1)`); err == nil {
		t.Error("отрицательный баланс принят")
	}

	// Одна открытая задача на клиента держится индексом, закрытых может быть сколько угодно.
	const insertTask = `INSERT INTO retention_tasks (customer_id, customer_email, status, outcome, assignee, due_at, baseline, created_by, created_at, updated_by, updated_at)
		VALUES (1, 'a@example.test', $1, $2, 'u', now(), '{}', 'u', now(), 'u', now())`
	for _, c := range []struct {
		status, outcome string
		ok              bool
	}{{"in_work", "", true}, {"in_work", "", false}, {"closed", "renewed", true}, {"closed", "refused", true}} {
		var outcome *string
		if c.outcome != "" {
			outcome = &c.outcome
		}
		if _, err := pool.Exec(ctx, insertTask, c.status, outcome); (err == nil) != c.ok {
			t.Errorf("задача %s/%s: err=%v, ждали ok=%v", c.status, c.outcome, err, c.ok)
		}
	}
	if _, err := pool.Exec(ctx, insertTask, "closed", nil); err == nil {
		t.Error("закрытая задача без итога принята")
	}

	if err := db.DownAll(ctx, dsn); err != nil {
		t.Fatalf("down: %v", err)
	}
	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_tables WHERE schemaname='public' AND tablename <> 'goose_db_version'`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("после down осталось таблиц: %d", left)
	}
	if err := db.Up(ctx, dsn); err != nil {
		t.Fatalf("повторный up: %v", err)
	}
}

func TestOrganizerSeedLoadsIntoSchema(t *testing.T) {
	dsn := dbtest.New(t)
	ctx := context.Background()
	if err := db.Up(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	seed, err := os.ReadFile("../../../../cases/analizator/002_seed.sql")
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	if _, err := conn.Exec(ctx, string(seed)); err != nil {
		t.Fatalf("сид не лёг в схему: %v", err)
	}
	for table, want := range map[string]int{"accounts": 10, "services": 30, "transactions": 100} {
		var n int
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM public."+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != want {
			t.Errorf("%s: %d строк, ждали %d", table, n, want)
		}
	}
}

package db_test

import (
	"context"
	"testing"

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
	for _, table := range []string{"accounts", "services", "transactions", "snapshots", "model_settings", "users", "sessions"} {
		var ok bool
		if err := pool.QueryRow(ctx, "SELECT to_regclass('public.'||$1) IS NOT NULL", table).Scan(&ok); err != nil || !ok {
			t.Errorf("таблицы %s нет после up (err=%v)", table, err)
		}
	}
	// Ограничения организаторов сохранены: баланс не уходит в минус.
	if _, err := pool.Exec(ctx, `INSERT INTO accounts (email, balance) VALUES ('a@example.test', -1)`); err == nil {
		t.Error("отрицательный баланс принят")
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

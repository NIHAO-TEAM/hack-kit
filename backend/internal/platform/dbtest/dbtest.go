// Package dbtest выдаёт тестам чистую базу PostgreSQL.
package dbtest

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// New создаёт отдельную базу на сервере из TEST_DATABASE_URL и возвращает её DSN.
// Без TEST_DATABASE_URL тест пропускается; в CI база есть, пропуск там считается падением.
func New(t testing.TB) string {
	t.Helper()
	admin := os.Getenv("TEST_DATABASE_URL")
	if admin == "" {
		t.Skip("TEST_DATABASE_URL не задан")
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("dbtest: подключение: %v", err)
	}
	name := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		_ = conn.Close(ctx)
		t.Fatalf("dbtest: создание базы: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name))
		_ = conn.Close(ctx)
	})
	u, err := url.Parse(admin)
	if err != nil {
		t.Fatalf("dbtest: разбор TEST_DATABASE_URL: %v", err)
	}
	u.Path = "/" + name
	return u.String()
}

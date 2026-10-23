package auth

import (
	"context"
	"testing"
	"time"

	"example.com/app/internal/auth/db"
	"example.com/app/internal/platform/clock"
	platformdb "example.com/app/internal/platform/db"
	"example.com/app/internal/platform/dbtest"
)

func newTestAuth(t *testing.T, cfg Config) (*AuthAPI, *db.Queries) {
	t.Helper()
	dsn := dbtest.New(t)
	ctx := context.Background()
	if err := platformdb.Up(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	pool, err := platformdb.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	a := New(pool, clock.NewFake(time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)), cfg)
	return a, db.New(pool)
}

func TestOnStartCreatesAccountsFromConfig(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		AdminLogin: "boss", AdminPassword: "boss-pass",
		DemoPasswords: map[string]string{RoleJury: "jury-pass", RoleAnalyst: "analyst-pass"},
	}
	a, q := newTestAuth(t, cfg)
	if err := a.OnStart(ctx); err != nil {
		t.Fatal(err)
	}

	for login, role := range map[string]string{"boss": RoleAdmin, "jury": RoleJury, "analyst": RoleAnalyst} {
		u, err := q.GetUserByLogin(ctx, login)
		if err != nil {
			t.Fatalf("%s: %v", login, err)
		}
		if u.Role != role {
			t.Errorf("%s: роль %q, ждали %q", login, u.Role, role)
		}
		if u.PasswordHash == "" || u.PasswordHash == "boss-pass" {
			t.Errorf("%s: пароль хранится не хешем", login)
		}
	}
	if _, err := q.GetUserByLogin(ctx, "marketer"); err == nil {
		t.Error("учётка marketer создана без пароля")
	}
}

func TestOnStartChangesPasswordWhenEnvChanges(t *testing.T) {
	ctx := context.Background()
	cfg := Config{DemoPasswords: map[string]string{RoleJury: "old-pass"}}
	a, q := newTestAuth(t, cfg)
	if err := a.OnStart(ctx); err != nil {
		t.Fatal(err)
	}
	before, _ := q.GetUserByLogin(ctx, "jury")

	// Повторный старт с тем же паролем ничего не переписывает.
	if err := a.OnStart(ctx); err != nil {
		t.Fatal(err)
	}
	same, _ := q.GetUserByLogin(ctx, "jury")
	if same.PasswordHash != before.PasswordHash || same.ID != before.ID {
		t.Error("повторный старт переписал неизменённую учётку")
	}

	a.cfg.DemoPasswords = map[string]string{RoleJury: "new-pass"}
	a.svc.cfg = a.cfg
	if err := a.OnStart(ctx); err != nil {
		t.Fatal(err)
	}
	after, _ := q.GetUserByLogin(ctx, "jury")
	if after.ID != before.ID {
		t.Error("смена пароля создала другую учётку")
	}
	if VerifyPassword("old-pass", after.PasswordHash) || !VerifyPassword("new-pass", after.PasswordHash) {
		t.Error("пароль не сменился вместе с переменной")
	}
}

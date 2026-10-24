// Package db открывает пул соединений PostgreSQL и применяет миграции.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // драйвер database/sql для goose
	"github.com/pressly/goose/v3"

	"example.com/app/migrations"
)

const (
	defaultMaxConns    = 16
	defaultMaxConnLife = 30 * time.Minute
)

// poolConfig разбирает DSN и задаёт размер пула из DB_MAX_CONNS (по умолчанию 16).
func poolConfig(dsn string) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = defaultMaxConns
	if v := os.Getenv("DB_MAX_CONNS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 1000 {
			return nil, fmt.Errorf("DB_MAX_CONNS=%q: нужно целое от 1 до 1000", v)
		}
		cfg.MaxConns = int32(n)
	}
	cfg.MaxConnLifetime = defaultMaxConnLife
	return cfg, nil
}

// Open создаёт пул соединений.
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := poolConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("db: пул: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("db: пул: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}
	return pool, nil
}

func provider(sqlDB *sql.DB) (*goose.Provider, error) {
	return goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
}

// Up применяет все новые миграции.
func Up(ctx context.Context, dsn string) error {
	return run(dsn, func(p *goose.Provider) error {
		_, err := p.Up(ctx)
		return err
	})
}

// DownAll откатывает все миграции (тесты).
func DownAll(ctx context.Context, dsn string) error {
	return run(dsn, func(p *goose.Provider) error {
		_, err := p.DownTo(ctx, 0)
		return err
	})
}

func run(dsn string, f func(*goose.Provider) error) error {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("db: открытие: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()
	p, err := provider(sqlDB)
	if err != nil {
		return fmt.Errorf("db: goose: %w", err)
	}
	if err := f(p); err != nil {
		return fmt.Errorf("db: миграции: %w", err)
	}
	return nil
}

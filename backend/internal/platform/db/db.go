// Package db открывает пул соединений PostgreSQL и применяет миграции.
package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // драйвер database/sql для goose
	"github.com/pressly/goose/v3"

	"example.com/app/migrations"
)

// Open создаёт пул соединений.
func Open(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
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
	defer sqlDB.Close()
	p, err := provider(sqlDB)
	if err != nil {
		return fmt.Errorf("db: goose: %w", err)
	}
	if err := f(p); err != nil {
		return fmt.Errorf("db: миграции: %w", err)
	}
	return nil
}

package db

import (
	"testing"
	"time"
)

func TestPoolConfigMaxConns(t *testing.T) {
	const dsn = "postgres://u:p@localhost:5432/d?sslmode=disable"
	for _, c := range []struct {
		env  string
		want int32
		bad  bool
	}{{"", 16, false}, {"32", 32, false}, {"0", 0, true}, {"abc", 0, true}, {"5000", 0, true}} {
		t.Setenv("DB_MAX_CONNS", c.env)
		cfg, err := poolConfig(dsn)
		if c.bad {
			if err == nil {
				t.Errorf("DB_MAX_CONNS=%q принят", c.env)
			}
			continue
		}
		if err != nil || cfg.MaxConns != c.want {
			t.Errorf("DB_MAX_CONNS=%q: MaxConns=%v err=%v, ждали %d", c.env, cfg.MaxConns, err, c.want)
		}
		if cfg.MaxConnLifetime != 30*time.Minute {
			t.Errorf("MaxConnLifetime=%v", cfg.MaxConnLifetime)
		}
	}
}

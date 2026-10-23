package source

import (
	"testing"
	"time"
)

func TestBackoff(t *testing.T) {
	for _, c := range []struct {
		interval time.Duration
		fails    int
		want     time.Duration
	}{
		{time.Minute, 0, time.Minute},
		{time.Minute, 1, 2 * time.Minute},
		{time.Minute, 2, 4 * time.Minute},
		{time.Minute, 3, maxBackoff},
		{time.Minute, 30, maxBackoff},
		{10 * time.Minute, 3, 10 * time.Minute},
	} {
		if got := backoff(c.interval, c.fails); got != c.want {
			t.Errorf("backoff(%v, %d) = %v, ждали %v", c.interval, c.fails, got, c.want)
		}
	}
}

package clock_test

import (
	"testing"
	"time"

	"example.com/app/internal/platform/clock"
)

func TestFakeAdvance(t *testing.T) {
	start := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	c := clock.NewFake(start)
	if !c.Now().Equal(start) {
		t.Fatalf("Now = %v", c.Now())
	}
	c.Advance(36 * time.Hour)
	if want := start.Add(36 * time.Hour); !c.Now().Equal(want) {
		t.Errorf("после Advance Now = %v, ждали %v", c.Now(), want)
	}
}

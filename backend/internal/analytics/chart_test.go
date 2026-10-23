package analytics

import (
	"testing"
	"time"
)

func TestBuildChurnChart(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC) // среда; неделя с 19.10
	day := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }
	svcs := []chartService{
		{ExpiresAt: day(10, 5), Deleted: true},  // удалена 10.10 → неделя с 5.10
		{ExpiresAt: day(10, 10), Deleted: true}, // удалена 15.10 → неделя с 12.10
		{ExpiresAt: day(10, 11), Deleted: true}, // удалена 16.10 → неделя с 12.10
		{ExpiresAt: day(7, 1), Deleted: true},   // старше горизонта
		{ExpiresAt: day(10, 22), P: 0.6},        // текущая неделя
		{ExpiresAt: day(10, 23), P: 0.5},        // текущая неделя
		{ExpiresAt: day(11, 3), P: 0.9},         // неделя с 2.11
		{ExpiresAt: day(10, 20), P: 0.9},        // срок прошёл, не прогноз
		{ExpiresAt: day(12, 30), P: 0.9},        // дальше горизонта
	}
	pts := buildChurnChart(svcs, now)
	if len(pts) != 17 {
		t.Fatalf("точек %d, ждали 17", len(pts))
	}
	if got := pts[0].WeekStart.Format("2006-01-02"); got != "2026-08-24" {
		t.Errorf("первая неделя %s", got)
	}
	check := func(i int, wantA, wantF *int) {
		t.Helper()
		eq := func(a, b *int) bool { return (a == nil && b == nil) || (a != nil && b != nil && *a == *b) }
		if !eq(pts[i].Actual, wantA) || !eq(pts[i].Forecast, wantF) {
			t.Errorf("точка %d: actual %v forecast %v", i, pts[i].Actual, pts[i].Forecast)
		}
	}
	v := func(n int) *int { return &n }
	check(0, v(0), nil)
	check(6, v(1), nil)  // неделя с 5.10
	check(7, v(2), nil)  // неделя с 12.10
	check(8, v(0), v(1)) // текущая: 0.6+0.5=1.1 → 1
	check(10, nil, v(1)) // неделя с 2.11
	check(16, nil, v(0))
}

package risk

import (
	"math"
	"testing"
	"time"
)

var asOfT = ts("2026-10-19T15:00:00Z")

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestPwait(t *testing.T) {
	day := 24 * time.Hour
	cases := []struct {
		name   string
		status string
		exp    time.Time
		want   float64
		ok     bool
	}{
		{"до удаления 2 дня", StatusProlongWait, asOfT.Add(-3 * day), 0.6, true},
		{"срок вышел", StatusProlongWait, asOfT.Add(-6 * day), 1, true},
		{"ровно в момент удаления", StatusProlongWait, asOfT.Add(-5 * day), 1, true},
		{"только вошла в ожидание", StatusProlongWait, asOfT, 0, true},
		{"не prolong_wait", StatusActive, asOfT, 0, false},
	}
	for _, c := range cases {
		f, reason, ok := pwait{}.Eval(ServiceCtx{AsOf: asOfT, Service: Service{Status: c.status, ExpiresAt: c.exp}})
		if ok != c.ok || (ok && !near(f, c.want)) {
			t.Errorf("%s: f=%v ok=%v, ждали %v %v", c.name, f, ok, c.want, c.ok)
		}
		if ok && reason == "" {
			t.Errorf("%s: нет причины", c.name)
		}
	}
}

func TestNoauto(t *testing.T) {
	day := 24 * time.Hour
	cases := []struct {
		name string
		auto bool
		exp  time.Time
		want float64
		ok   bool
	}{
		{"до срока 8 дней", false, asOfT.Add(8 * day), 0, true},
		{"ровно 7 дней", false, asOfT.Add(7 * day), 0, true},
		{"3,5 дня", false, asOfT.Add(84 * time.Hour), 0.5, true},
		{"срок через 30 минут", false, asOfT.Add(30 * time.Minute), 1 - (30.0/1440)/7, true},
		{"срок прошёл", false, asOfT.Add(-day), 1, true},
		{"автопродление включено", true, asOfT, 0, false},
	}
	for _, c := range cases {
		f, _, ok := noauto{}.Eval(ServiceCtx{AsOf: asOfT, Service: Service{AutoProlong: c.auto, ExpiresAt: c.exp}})
		if ok != c.ok || (ok && !near(f, c.want)) {
			t.Errorf("%s: f=%v ok=%v, ждали %v %v", c.name, f, ok, c.want, c.ok)
		}
	}
}

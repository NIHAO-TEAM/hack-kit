//go:build integration

package app_test

import (
	"context"
	"testing"
	"time"
)

// Статус источника отдаёт проверки по последнему пересчёту; на наборе организаторов срабатывает stale_status.
func TestStatusQualityChecks(t *testing.T) {
	r := newRig(t, true)
	rc := r.StartRecalc(t)
	waitLatest(t, rc, "пересчёт при старте", func() bool { return rc.Engine.Latest() != nil })
	c := r.As(t, "analyst")
	var checks map[string]bool
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		res, err := c.GetSourceStatusWithResponse(context.Background())
		if err != nil || res.JSON200 == nil {
			t.Fatalf("статус: %v %v", err, res)
		}
		if len(res.JSON200.Checks) == 3 {
			checks = map[string]bool{}
			for _, q := range res.JSON200.Checks {
				checks[q.Id] = q.Ok
				if q.Id == "stale_status" {
					if q.Ok || q.Items == nil || len(*q.Items) != 6 || q.Message == nil || *q.Message != "6 услуг в ожидании оплаты дольше 5 дней" {
						t.Errorf("stale_status: %+v", q)
					}
				}
			}
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if checks == nil || checks["stale_status"] || !checks["blocked_future"] || !checks["suspended_expired"] {
		t.Errorf("проверки: %v", checks)
	}
	// Сводка и экран из одного расчёта: счётчик stale_status совпадает с qualityCounts.
	if n := rc.Overview().QualityCounts["ошибка данных: в ожидании оплаты дольше 5 дней"]; n != 6 {
		t.Errorf("qualityCounts: %d", n)
	}
}

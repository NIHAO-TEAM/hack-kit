//go:build integration

package source_test

import (
	"testing"

	"example.com/app/internal/apitest"
	"example.com/app/internal/platform/api/gen"
)

// Поток по умолчанию, 2 500 событий без пауз: отчёт заполнен, пересчёт с нуля совпал до цента.
func TestStreamReportOn2500Events(t *testing.T) {
	s := apitest.New(t)
	s.StartRecalc(t)
	c := s.As(t, "jury")
	if res := put(t, c, organizers(t)); res.JSON200 == nil {
		t.Fatalf("снимок: %s", res.Body)
	}
	if res := startStream(t, c, 2500, 0, 1); res.JSON202 == nil {
		t.Fatalf("запуск: %d %s", res.StatusCode(), res.Body)
	}
	st := waitStream(t, c)
	if st.State != gen.Done || st.Report == nil {
		t.Fatalf("состояние %s, ошибка %v", st.State, st.Error)
	}
	r := st.Report
	t.Logf("события %d/%d, не применено %d, %d мс, %.1f событий/с", r.Applied, r.Events, r.Failed, r.DurationMs, r.EventsPerSecond)
	t.Logf("применение p50 %.2f p95 %.2f max %.2f мс; пересчётов %d; до очереди p95 %.1f мс", r.ApplyP50Ms, r.ApplyP95Ms, r.ApplyMaxMs, r.Recalcs, *r.QueueLagP95Ms)
	t.Logf("уровни до %+v → после %+v; потеря %d → %d ¢; смен уровня %v", r.LevelsBefore, r.LevelsAfter, r.LossBeforeCents, r.LossAfterCents, r.LevelChanges)
	t.Logf("согласованность %+v", r.Consistency)
	if r.Events != 2500 || r.Applied != 2500 || r.Failed != 0 {
		t.Errorf("события %d, применено %d, не применено %d", r.Events, r.Applied, r.Failed)
	}
	if !r.Consistency.Ok || r.Consistency.DiffCents != 0 {
		t.Errorf("согласованность: %+v", r.Consistency)
	}
	if r.Recalcs < 1 || r.Recalcs > 2500 {
		t.Errorf("пересчётов %d", r.Recalcs)
	}
}

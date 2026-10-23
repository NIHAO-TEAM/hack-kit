package risk

import (
	"testing"
	"time"
)

func TestQualityChecks(t *testing.T) {
	day := 24 * time.Hour
	cases := []struct {
		name  string
		check Check
		sv    Service
		hit   bool
	}{
		{"prolong_wait старше 5 дней", qualityStale{}, Service{Status: StatusProlongWait, ExpiresAt: asOfT.Add(-6 * day)}, true},
		{"prolong_wait свежий", qualityStale{}, Service{Status: StatusProlongWait, ExpiresAt: asOfT.Add(-2 * day)}, false},
		{"suspended с истёкшим сроком", qualitySuspended{}, Service{Status: StatusSuspended, ExpiresAt: asOfT.Add(-day)}, true},
		{"suspended в срок", qualitySuspended{}, Service{Status: StatusSuspended, ExpiresAt: asOfT.Add(day)}, false},
		{"blocked со сроком в будущем", qualityBlocked{}, Service{Status: StatusBlocked, ExpiresAt: asOfT.Add(day)}, true},
		{"blocked с прошедшим сроком", qualityBlocked{}, Service{Status: StatusBlocked, ExpiresAt: asOfT.Add(-day)}, false},
	}
	for _, c := range cases {
		if _, hit := c.check.Eval(ServiceCtx{AsOf: asOfT, Service: c.sv}); hit != c.hit {
			t.Errorf("%s: hit=%v", c.name, hit)
		}
	}
}

// На наборе организаторов шесть услуг с устаревшим статусом; проверки на скор не влияют (golden).
func TestQualityOrganizers(t *testing.T) {
	var snap Snapshot
	readJSON(t, "../../../cases/analizator/data.json", &snap)
	res := Score(snap, Defaults(), ts("2026-10-19T15:00:00Z"))
	if got := res.Summary.QualityCounts["ошибка данных: в ожидании оплаты дольше 5 дней"]; got != 6 {
		t.Errorf("ошибка данных: в ожидании оплаты дольше 5 дней: %d, ждали 6 (%v)", got, res.Summary.QualityCounts)
	}
}

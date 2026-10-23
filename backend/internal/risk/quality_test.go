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

func TestChecksResult(t *testing.T) {
	var snap Snapshot
	readJSON(t, "../../../cases/analizator/data.json", &snap)
	res := Score(snap, Defaults(), ts("2026-10-19T15:00:00Z"))
	if len(res.Checks) != 3 {
		t.Fatalf("проверок: %+v", res.Checks)
	}
	byID := map[string]CheckResult{}
	for _, c := range res.Checks {
		byID[c.ID] = c
	}
	st := byID["stale_status"]
	if st.OK || st.Count != 6 || len(st.Items) != 6 || st.Message != "6 услуг в ожидании оплаты дольше 5 дней" {
		t.Errorf("stale_status: %+v", st)
	}
	if st.Items[0].ServiceID == 0 || st.Items[0].AccountID == 0 {
		t.Errorf("пустой элемент: %+v", st.Items[0])
	}
	if c := byID["blocked_future"]; !c.OK && c.Count == 0 || c.OK && c.Message != "Нет услуг в статусе «заблокирована» со сроком в будущем" {
		t.Errorf("blocked_future: %+v", c)
	}
}

func TestServicesWord(t *testing.T) {
	for n, want := range map[int]string{1: "услуга", 2: "услуги", 5: "услуг", 11: "услуг", 21: "услуга", 12: "услуг", 24: "услуги"} {
		if got := servicesWord(n); got != want {
			t.Errorf("%d: %s", n, got)
		}
	}
}

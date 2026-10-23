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

// blocked со сроком в будущем не живая услуга, но проверка качества её видит; скор и деньги не меняются.
func TestQualityBlockedFutureOutsideScore(t *testing.T) {
	day := 24 * time.Hour
	base := Snapshot{
		AsOf:     asOfT,
		Accounts: []Account{{ID: 1, Balance: 5000}, {ID: 2, Balance: 5000}},
		Services: []Service{
			{ID: 10, OwnerID: 1, PaymentTerm: "month", AutoProlong: true, Status: StatusActive,
				ExpiresAt: asOfT.Add(40 * day), Price: 900},
			{ID: 20, OwnerID: 2, PaymentTerm: "month", Status: StatusBlocked,
				ExpiresAt: asOfT.Add(5 * day), Price: 700},
		},
	}
	with := Score(base, Defaults(), asOfT)
	if got := with.Summary.QualityCounts["заблокирована, срок в будущем"]; got != 1 {
		t.Errorf("blocked_future: %d, ждали 1 (%v)", got, with.Summary.QualityCounts)
	}
	if q := with.Customers[1].Quality; len(q) != 1 || q[0].ServiceID != 20 {
		t.Errorf("флаг у клиента 2: %+v", q)
	}

	// та же услуга с прошедшим сроком: флага нет, а скор, уровни, деньги совпадают
	past := base
	past.Services = append([]Service(nil), base.Services...)
	past.Services[1].ExpiresAt = asOfT.Add(-day)
	without := Score(past, Defaults(), asOfT)
	if without.Summary.QualityCounts["заблокирована, срок в будущем"] != 0 {
		t.Errorf("срок в прошлом: %v", without.Summary.QualityCounts)
	}
	for i := range with.Customers {
		a, b := with.Customers[i], without.Customers[i]
		if a.State != b.State || a.ExpectedLossCents != b.ExpectedLossCents || a.ExposureCents != b.ExposureCents ||
			len(a.Services) != len(b.Services) {
			t.Errorf("клиент %d: скор или деньги изменились: %+v / %+v", a.ID, a, b)
		}
	}
	if with.Summary.Levels[StateLost] != 1 {
		t.Errorf("уровни: %v", with.Summary.Levels)
	}
}

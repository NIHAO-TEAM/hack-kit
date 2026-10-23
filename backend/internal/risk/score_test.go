package risk

import (
	"math"
	"testing"
	"time"
)

func twoServiceSnapshot() Snapshot {
	return Snapshot{
		AsOf: asOfT,
		Accounts: []Account{
			{ID: 1, Balance: 100000},
			{ID: 2, Balance: 5000},
			{ID: 3, Balance: 5000},
			{ID: 4, Balance: 5000},
		},
		Services: []Service{
			// pwait: срок вышел 3 дня назад → до удаления 2 дня, f = 0,6, цена 1000 за месяц
			{ID: 10, OwnerID: 1, PaymentTerm: "month", AutoProlong: true, Status: StatusProlongWait,
				ExpiresAt: asOfT.Add(-3 * 24 * time.Hour), Price: 1000},
			// noauto: срок через 0 дней → f = 1, месяц, цена 2000
			{ID: 11, OwnerID: 1, PaymentTerm: "month", AutoProlong: false, Status: StatusActive,
				ExpiresAt: asOfT, Price: 2000},
			// клиент 2: только удалённые услуги
			{ID: 20, OwnerID: 2, PaymentTerm: "month", Status: StatusDeleted, ExpiresAt: asOfT, Price: 700},
			// клиент 3: нет услуг
			// клиент 4: здоровая услуга, автопродление, срок далеко
			{ID: 40, OwnerID: 4, PaymentTerm: "month", AutoProlong: true, Status: StatusActive,
				ExpiresAt: asOfT.Add(40 * 24 * time.Hour), Price: 900},
		},
	}
}

func TestScoreTwoServices(t *testing.T) {
	r := coreScore(twoServiceSnapshot(), Defaults(), asOfT)
	if len(r.Customers) != 4 {
		t.Fatalf("клиентов %d", len(r.Customers))
	}
	c := r.Customers[0]
	// ручной расчёт: услуга 10: P = 0,8·0,6 = 0,48; услуга 11: P = 0,55·1 = 0,55; max = 0,55
	if c.State != StateScored || c.Score == nil || *c.Score != 55 || *c.Level != LevelMedium {
		t.Fatalf("клиент 1: %+v score=%v", c, c.Score)
	}
	s10, s11 := c.Services[0], c.Services[1]
	if !near(s10.P, 0.48) || !near(s11.P, 0.55) {
		t.Fatalf("P: %v %v", s10.P, s11.P)
	}
	// R: prolong_wait — 1 списание (1000); месяц от asOf — 1 списание (2000)
	if s10.RCents != 1000 || s11.RCents != 2000 {
		t.Fatalf("R: %d %d", s10.RCents, s11.RCents)
	}
	if s10.LossCents != 480 || s11.LossCents != 1100 || c.ExpectedLossCents != 1580 || c.ExposureCents != 3000 {
		t.Fatalf("потери: %d %d %d %d", s10.LossCents, s11.LossCents, c.ExpectedLossCents, c.ExposureCents)
	}
	if len(s11.Features) != 1 || s11.Features[0].ID != "noauto" || s11.Features[0].Reason == "" {
		t.Fatalf("причины: %+v", s11.Features)
	}
	if h := s10.Features[0]; h.ID != "pwait" || !near(h.Contribution, 0.48) || !near(h.Weight, 0.8) {
		t.Fatalf("вклад: %+v", h)
	}
}

func TestScoreStatesAndSummary(t *testing.T) {
	r := coreScore(twoServiceSnapshot(), Defaults(), asOfT)
	if c := r.Customers[1]; c.State != StateLost || c.Score != nil || c.Level != nil || c.LostCents != 700 {
		t.Errorf("lost: %+v", c)
	}
	if c := r.Customers[2]; c.State != StateUnscored || c.Score != nil {
		t.Errorf("unscored: %+v", c)
	}
	// клиент 4: noauto не применим (автопродление), P = 0 → low
	if c := r.Customers[3]; c.State != StateScored || *c.Level != LevelLow || *c.Score != 0 || c.ExpectedLossCents != 0 {
		t.Errorf("low: %+v", c)
	}
	s := r.Summary
	if s.Levels[LevelMedium] != 1 || s.Levels[LevelLow] != 1 || s.Levels[StateLost] != 1 || s.Levels[StateUnscored] != 1 || s.Levels[LevelHigh] != 0 {
		t.Errorf("уровни: %v", s.Levels)
	}
	// «под угрозой» — только high и medium: клиент 1 (3000), low не входит
	if s.ExpectedLossCents != 1580 || s.ExposureCents != 3000 || s.LostCents != 700 {
		t.Errorf("сводка: %+v", s)
	}
}

func TestScoreThresholds(t *testing.T) {
	snap := Snapshot{Accounts: []Account{{ID: 1}}, Services: []Service{
		{ID: 1, OwnerID: 1, PaymentTerm: "month", Status: StatusProlongWait, AutoProlong: true, ExpiresAt: asOfT.Add(-10 * 24 * time.Hour), Price: 100},
	}}
	st := Defaults() // P = 0,8 → скор 80
	st.High, st.Medium = 80, 50
	if r := coreScore(snap, st, asOfT); *r.Customers[0].Level != LevelHigh {
		t.Errorf("ровно на пороге high: %v", *r.Customers[0].Level)
	}
	st.High, st.Medium = 90, 80
	if r := coreScore(snap, st, asOfT); *r.Customers[0].Level != LevelMedium {
		t.Errorf("ровно на пороге medium: %v", *r.Customers[0].Level)
	}
	st.Medium = 81
	if r := coreScore(snap, st, asOfT); *r.Customers[0].Level != LevelLow {
		t.Errorf("ниже medium: %v", *r.Customers[0].Level)
	}
}

func TestRound2(t *testing.T) {
	for in, want := range map[float64]float64{56.0199999: 56.02, 0: 0, 99.999: 100} {
		if got := round2(in); got != want {
			t.Errorf("round2(%v) = %v, ждали %v", in, got, want)
		}
	}
}

type fakeCheck struct{}

func (fakeCheck) ID() string { return "fake" }
func (fakeCheck) Eval(ctx ServiceCtx) (string, bool) {
	return "проверка сработала", ctx.Service.ID == 11
}

type fakeAction struct{}

func (fakeAction) ID() string               { return "top_up" }
func (fakeAction) Applies(CustomerCtx) bool { return true }
func (fakeAction) Title(CustomerCtx) string { return "Включить автопродление" }
func (fakeAction) Apply(s Snapshot, id int64) Snapshot {
	out := s
	out.Services = append([]Service(nil), s.Services...)
	for i := range out.Services {
		if out.Services[i].OwnerID == id {
			out.Services[i].AutoProlong = true
		}
	}
	return out
}

func TestChecksAndActionsHooks(t *testing.T) {
	snap := twoServiceSnapshot()
	r := scoreWith(snap, Defaults(), asOfT, features.all(), []Check{fakeCheck{}}, []Action{fakeAction{}})
	if r.Summary.QualityCounts["проверка сработала"] != 1 || len(r.Customers[0].Services[1].Quality) != 1 {
		t.Errorf("качество: %+v", r.Summary.QualityCounts)
	}
	c := r.Customers[0]
	if len(c.Actions) != 1 {
		t.Fatalf("действия: %+v", c.Actions)
	}
	a := c.Actions[0]
	// после: услуга 11 с автопродлением → P = 0; остаётся pwait 0,48 → скор 48, потеря 480; было 1580
	if a.LossDeltaCents != -1100 || *a.ScoreBefore != 55 || *a.ScoreAfter != 48 || *a.LevelAfter != LevelLow {
		t.Errorf("эффект: %+v before=%v after=%v", a, *a.ScoreBefore, *a.ScoreAfter)
	}
	if snap.Services[1].AutoProlong {
		t.Error("действие изменило исходный снимок")
	}
	if r2 := coreScore(snap, Defaults(), asOfT); len(r2.Customers[0].Actions) != 0 {
		t.Error("пустой реестр действий должен давать пустой список")
	}
}

func TestScoreDeterministic(t *testing.T) {
	a := coreScore(twoServiceSnapshot(), Defaults(), asOfT)
	b := coreScore(twoServiceSnapshot(), Defaults(), asOfT)
	if a.Summary.ExpectedLossCents != b.Summary.ExpectedLossCents || *a.Customers[0].Score != *b.Customers[0].Score {
		t.Error("недетерминированный результат")
	}
}

// Свойства на 200 случайных снимках с фиксированным seed: скор в [0, 100], рост веса не уменьшает скор.
func TestScoreProperties(t *testing.T) {
	rng := newLCG(42)
	statuses := []string{StatusActive, StatusSuspended, StatusProlongWait, StatusDeleted, StatusBlocked}
	terms := []string{"hour", "week", "month", "quarter_year", "half_year", "year"}
	for n := 0; n < 200; n++ {
		snap := Snapshot{}
		for a := int64(1); a <= 3; a++ {
			snap.Accounts = append(snap.Accounts, Account{ID: a, Balance: rng.intn(100000)})
			for k := 0; k < int(rng.intn(4)); k++ {
				snap.Services = append(snap.Services, Service{
					ID: a*10 + int64(k), OwnerID: a, PaymentTerm: terms[rng.intn(int64(len(terms)))],
					AutoProlong: rng.intn(2) == 0, Status: statuses[rng.intn(int64(len(statuses)))],
					ExpiresAt: asOfT.Add(time.Duration(rng.intn(20*24*2)-20*24) * time.Hour),
					Price:     rng.intn(50000),
				})
			}
		}
		st := Defaults()
		before := coreScore(snap, st, asOfT)
		for _, id := range []string{"pwait", "noauto"} {
			st2 := Defaults()
			st2.Weights[id] = math.Min(1, st.Weights[id]+0.2)
			after := coreScore(snap, st2, asOfT)
			for i, c := range before.Customers {
				if c.Score == nil {
					continue
				}
				if *c.Score < 0 || *c.Score > 100 {
					t.Fatalf("скор вне [0, 100]: %v", *c.Score)
				}
				if *after.Customers[i].Score < *c.Score {
					t.Fatalf("рост веса %s уменьшил скор: %v → %v", id, *c.Score, *after.Customers[i].Score)
				}
			}
		}
	}
}

type lcg struct{ s uint64 }

func newLCG(seed uint64) *lcg { return &lcg{seed} }
func (l *lcg) intn(n int64) int64 {
	l.s = l.s*6364136223846793005 + 1442695040888963407
	return int64(l.s>>33) % n
}

// coreScore — расчёт только на признаках ядра (pwait, noauto): ручные расчёты этих тестов
// не зависят от признаков, которые добавляют другие изменения.
func coreScore(s Snapshot, st Settings, asOf time.Time) Result {
	return scoreWith(s, st, asOf, []Feature{pwait{}, noauto{}}, nil, nil)
}

package risk

import "testing"

// fundsStub — упрощённый funds: f = 1, пока «есть» меньше «нужно». Настоящий признак — в risk-features.
type fundsStub struct{}

func (fundsStub) ID() string             { return "funds" }
func (fundsStub) DefaultWeight() float64 { return 0.7 }
func (fundsStub) Eval(ctx ServiceCtx) (float64, string, bool) {
	if ctx.Funds.Have >= ctx.Funds.Need {
		return 0, "", true
	}
	return 1, "не хватает денег", true
}

func topUpSnapshot() Snapshot {
	return Snapshot{
		AsOf:     asOfT,
		Accounts: []Account{{ID: 1, Balance: 1000}, {ID: 2, Balance: 1000}},
		Services: []Service{
			{ID: 10, OwnerID: 1, PaymentTerm: "month", AutoProlong: true, Status: StatusActive,
				ExpiresAt: asOfT.AddDate(0, 0, 3), Price: 5000},
			{ID: 20, OwnerID: 2, PaymentTerm: "month", AutoProlong: true, Status: StatusActive,
				ExpiresAt: asOfT.AddDate(0, 0, 3), Price: 800},
		},
	}
}

func TestTopUpAction(t *testing.T) {
	snap := topUpSnapshot()
	r := scoreWith(snap, Defaults(), asOfT, []Feature{fundsStub{}}, nil, []Action{topUp{}})
	c := r.Customers[0]
	if len(c.Actions) != 1 {
		t.Fatalf("действия клиента 1: %+v", c.Actions)
	}
	a := c.Actions[0]
	if a.ID != "top_up" || a.Title != "Пополнить баланс на 40,00 €" {
		t.Errorf("действие: %+v", a)
	}
	// P = 0,7 → потеря 3500; после пополнения funds = 0 → потеря 0
	if a.LossDeltaCents != -3500 || *a.ScoreBefore != 70 || *a.ScoreAfter != 0 || *a.LevelAfter != LevelLow {
		t.Errorf("эффект: %+v before=%v after=%v", a, *a.ScoreBefore, *a.ScoreAfter)
	}
	if len(r.Customers[1].Actions) != 0 {
		t.Errorf("клиенту 2 хватает денег: %+v", r.Customers[1].Actions)
	}
	if snap.Accounts[0].Balance != 1000 {
		t.Error("действие изменило исходный снимок")
	}
}

func TestTopUpApplyAddsMissingAmount(t *testing.T) {
	out := topUp{}.Apply(topUpSnapshot(), 1)
	if out.Accounts[0].Balance != 5000 || out.Accounts[1].Balance != 1000 {
		t.Errorf("балансы: %+v", out.Accounts)
	}
}

func TestTopUpUsesPriceBasis(t *testing.T) {
	snap := topUpSnapshot()
	snap.Services[0].PaymentTerm = "hour"
	snap.Services[0].Price = 10
	st := Defaults()
	st.PriceBasis = PricePerMonth
	// 168 часов при цене 10 за 720 ч → 2,33 → 2 цента; «есть» 1000 хватает
	out := topUp{}.ApplyWith(snap, 1, st)
	if out.Accounts[0].Balance != 1000 {
		t.Errorf("баланс: %d", out.Accounts[0].Balance)
	}
}

func TestEurosText(t *testing.T) {
	for in, want := range map[int64]string{52984: "529,84", 5: "0,05", 100: "1,00", -106748: "-1067,48"} {
		if got := eurosText(in); got != want {
			t.Errorf("eurosText(%d) = %q, ждали %q", in, got, want)
		}
	}
}

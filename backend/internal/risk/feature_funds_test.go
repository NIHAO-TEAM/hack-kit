package risk

import "testing"

func TestFunds(t *testing.T) {
	cases := []struct {
		name       string
		need, have int64
		want       float64
		reason     string
	}{
		{"клиент 9: нужно 1 444,90, есть 915,06", 144490, 91506, 0.366697, "Нужно 1 444,90 € на 7 дней, на балансе 915,06 €"},
		{"хватает", 1000, 1000, 0, ""},
		{"нечего платить", 0, 0, 0, ""},
		{"баланс пуст", 500, 0, 1, "Нужно 5,00 € на 7 дней, на балансе 0,00 €"},
	}
	for _, c := range cases {
		f, reason, ok := funds{}.Eval(ServiceCtx{Funds: FundsCtx{Need: c.need, Have: c.have}})
		if !ok || (f-c.want > 5e-7 || c.want-f > 5e-7) || reason != c.reason {
			t.Errorf("%s: f=%v %q ok=%v", c.name, f, reason, ok)
		}
	}
}

func TestFundsBonusLocked(t *testing.T) {
	a := Account{ID: 1, Balance: 100, BonusBalance: 100000, BonusState: "locked"}
	sv := Service{ID: 1, OwnerID: 1, PaymentTerm: "hour", Status: StatusActive, Price: 100}
	got := accountFunds(a, []Service{sv}, asOfT, Defaults())
	if got.Have != 100 {
		t.Fatalf("заблокированный бонус учтён: %+v", got)
	}
	a.BonusState = "unlocked"
	if got := accountFunds(a, []Service{sv}, asOfT, Defaults()); got.Have != 100100 {
		t.Fatalf("открытый бонус не учтён: %+v", got)
	}
}

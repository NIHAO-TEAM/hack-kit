package risk

import "testing"

func TestBonusLocked(t *testing.T) {
	cases := []struct {
		name string
		acc  Account
		fc   FundsCtx
		want float64
	}{
		{"бонус закрыт, не хватает", Account{BonusBalance: 5000, BonusState: "locked"}, FundsCtx{Need: 100, Have: 50}, 1},
		{"бонус закрыт, хватает", Account{BonusBalance: 5000, BonusState: "locked"}, FundsCtx{Need: 100, Have: 100}, 0},
		{"бонус открыт", Account{BonusBalance: 5000, BonusState: "unlocked"}, FundsCtx{Need: 100, Have: 50}, 0},
		{"бонуса нет", Account{BonusState: "locked"}, FundsCtx{Need: 100, Have: 50}, 0},
	}
	for _, c := range cases {
		f, reason, ok := bonusLocked{}.Eval(ServiceCtx{Account: c.acc, Funds: c.fc})
		if ok != (c.want > 0) || f != c.want || (f > 0) != (reason != "") {
			t.Errorf("%s: f=%v %q ok=%v", c.name, f, reason, ok)
		}
	}
	if (bonusLocked{}).DefaultWeight() != 0 {
		t.Error("вес по умолчанию должен быть 0")
	}
}

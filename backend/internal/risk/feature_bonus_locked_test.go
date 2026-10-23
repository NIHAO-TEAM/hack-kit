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

// Вес 0 не меняет скор, вес 0,3 поднимает его у клиента с заблокированным бонусом и нехваткой средств.
func TestBonusLockedWeight(t *testing.T) {
	var snap Snapshot
	readJSON(t, "../../../cases/analizator/data.json", &snap)
	asOf := ts("2026-10-19T15:00:00Z")
	base := Score(snap, Defaults(), asOf)
	st := Defaults()
	st.Weights["bonus_locked"] = 0.3
	on := Score(snap, st, asOf)
	raised := 0
	for i, c := range base.Customers {
		o := on.Customers[i]
		if c.Score == nil {
			continue
		}
		if *o.Score < *c.Score {
			t.Errorf("клиент %d: скор упал %v → %v", c.ID, *c.Score, *o.Score)
		}
		if *o.Score > *c.Score {
			raised++
		}
	}
	if raised == 0 {
		t.Error("вес 0,3 никому не поднял скор")
	}
}

package risk

import (
	"reflect"
	"testing"
)

// Действие меняет данные одного клиента: остальные аккаунты и услуги остаются прежними, их числа не меняются.
func TestActionLeavesOtherCustomersUntouched(t *testing.T) {
	snap := topUpSnapshot()
	snap.Services[1].AutoProlong = false
	snap.Services[1].ExpiresAt = asOfT.AddDate(0, 0, 1)
	all := []Action{topUp{}, autoProlong{}, toBilling{}}
	fs := []Feature{fundsStub{}, noauto{}}

	for _, act := range all {
		out := act.Apply(snap, 1)
		if !reflect.DeepEqual(out.Accounts[1], snap.Accounts[1]) || !reflect.DeepEqual(out.Services[1], snap.Services[1]) {
			t.Errorf("%s изменило данные клиента 2", act.ID())
		}
	}

	with := scoreWith(snap, Defaults(), asOfT, fs, nil, all)
	without := scoreWith(snap, Defaults(), asOfT, fs, nil, nil)
	for i := range with.Customers {
		a, b := with.Customers[i], without.Customers[i]
		a.Actions, b.Actions = nil, nil
		if !reflect.DeepEqual(a, b) {
			t.Errorf("клиент %d: числа изменились из-за действий", a.ID)
		}
	}
	if len(with.Customers[0].Actions) == 0 || len(with.Customers[1].Actions) == 0 {
		t.Fatalf("действия не посчитаны: %+v / %+v", with.Customers[0].Actions, with.Customers[1].Actions)
	}
	if with.Summary.ExpectedLossCents != without.Summary.ExpectedLossCents {
		t.Error("действия изменили сводку")
	}
}

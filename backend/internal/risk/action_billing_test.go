package risk

import "testing"

func TestToBillingApplies(t *testing.T) {
	live := Service{ID: 1, OwnerID: 1, Status: StatusActive, AutoProlong: true}
	cases := []struct {
		name  string
		funds FundsCtx
		extra Service
		want  bool
	}{
		{"автопродление, денег хватает, deleted", FundsCtx{Need: 100, Have: 500}, Service{AutoProlong: true, Status: StatusDeleted}, true},
		{"автопродление, денег хватает, blocked", FundsCtx{Need: 100, Have: 100}, Service{AutoProlong: true, Status: StatusBlocked}, true},
		{"автопродление, денег хватает, prolong_wait", FundsCtx{Need: 100, Have: 500}, Service{AutoProlong: true, Status: StatusProlongWait}, true},
		{"денег не хватает", FundsCtx{Need: 600, Have: 500}, Service{AutoProlong: true, Status: StatusDeleted}, false},
		{"автопродление выключено", FundsCtx{Need: 100, Have: 500}, Service{AutoProlong: false, Status: StatusDeleted}, false},
		{"услуга живая и продлена", FundsCtx{Need: 100, Have: 500}, Service{AutoProlong: true, Status: StatusSuspended}, false},
	}
	for _, tc := range cases {
		ctx := CustomerCtx{Funds: tc.funds, Services: []Service{live, tc.extra}}
		if got := (toBilling{}).Applies(ctx); got != tc.want {
			t.Errorf("%s: %v, ждали %v", tc.name, got, tc.want)
		}
	}
}

func TestToBillingHasZeroEffect(t *testing.T) {
	snap := Snapshot{
		AsOf:     asOfT,
		Accounts: []Account{{ID: 1, Balance: 100000}},
		Services: []Service{
			{ID: 10, OwnerID: 1, PaymentTerm: "month", AutoProlong: false, Status: StatusActive,
				ExpiresAt: asOfT, Price: 2000},
			{ID: 11, OwnerID: 1, PaymentTerm: "month", AutoProlong: true, Status: StatusDeleted,
				ExpiresAt: asOfT, Price: 900},
		},
	}
	r := scoreWith(snap, Defaults(), asOfT, features.all(), nil, []Action{toBilling{}, autoProlong{}})
	acts := r.Customers[0].Actions
	if len(acts) != 2 || acts[0].ID != "enable_autoprolong" || acts[1].ID != "to_billing" || acts[1].LossDeltaCents != 0 {
		t.Errorf("действия (по эффекту, «к биллингу» последним): %+v", acts)
	}
}

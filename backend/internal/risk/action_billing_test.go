package risk

import "testing"

func TestToBillingApplies(t *testing.T) {
	fail := func(id int64) Transaction {
		return Transaction{ID: id, Type: "prolong", Status: "not_performed", ServiceID: 2}
	}
	wait := Service{ID: 2, AutoProlong: true, Status: StatusProlongWait}
	cases := []struct {
		name  string
		funds FundsCtx
		sv    Service
		txs   []Transaction
		want  bool
	}{
		{"prolong_wait, денег хватает, есть not_performed", FundsCtx{Need: 100, Have: 500}, wait, []Transaction{fail(1)}, true},
		{"prolong_wait, попыток нет (клиент 10)", FundsCtx{Need: 100, Have: 500}, wait, nil, false},
		{"prolong_wait, продление прошло", FundsCtx{Need: 100, Have: 500}, wait,
			[]Transaction{{Type: "prolong", Status: "performed", ServiceID: 2}}, false},
		{"not_performed чужого типа", FundsCtx{Need: 100, Have: 500}, wait,
			[]Transaction{{Type: "replenish", Status: "not_performed", ServiceID: 2}}, false},
		{"not_performed у другой услуги", FundsCtx{Need: 100, Have: 500}, wait,
			[]Transaction{{Type: "prolong", Status: "not_performed", ServiceID: 99}}, false},
		{"денег не хватает", FundsCtx{Need: 600, Have: 500}, wait, []Transaction{fail(1)}, false},
		{"автопродление выключено", FundsCtx{Need: 100, Have: 500}, Service{ID: 2, Status: StatusProlongWait}, []Transaction{fail(1)}, false},
		{"deleted не биллинг", FundsCtx{Need: 100, Have: 500}, Service{ID: 2, AutoProlong: true, Status: StatusDeleted}, []Transaction{fail(1)}, false},
		{"blocked не биллинг", FundsCtx{Need: 100, Have: 500}, Service{ID: 2, AutoProlong: true, Status: StatusBlocked}, []Transaction{fail(1)}, false},
	}
	for _, tc := range cases {
		ctx := CustomerCtx{Funds: tc.funds, Services: []Service{tc.sv}, Transactions: tc.txs}
		if got := (toBilling{}).Applies(ctx); got != tc.want {
			t.Errorf("%s: %v, ждали %v", tc.name, got, tc.want)
		}
	}
}

func TestToBillingTitleCountsAttempts(t *testing.T) {
	ctx := CustomerCtx{
		Services: []Service{{ID: 2, AutoProlong: true, Status: StatusProlongWait}},
		Transactions: []Transaction{
			{Type: "prolong", Status: "not_performed", ServiceID: 2},
			{Type: "prolong", Status: "not_performed", ServiceID: 2},
		},
	}
	want := "К биллингу: 2 продлений не прошли при достаточном балансе"
	if got := (toBilling{}).Title(ctx); got != want {
		t.Errorf("заголовок %q, ждали %q", got, want)
	}
}

func TestToBillingHasZeroEffect(t *testing.T) {
	snap := Snapshot{
		AsOf:     asOfT,
		Accounts: []Account{{ID: 1, Balance: 100000}},
		Services: []Service{
			{ID: 10, OwnerID: 1, PaymentTerm: "month", AutoProlong: false, Status: StatusActive,
				ExpiresAt: asOfT, Price: 2000},
			{ID: 11, OwnerID: 1, PaymentTerm: "month", AutoProlong: true, Status: StatusProlongWait,
				ExpiresAt: asOfT, Price: 900},
		},
		Transactions: []Transaction{{ID: 1, Type: "prolong", Status: "not_performed", ServiceID: 11, CreatedAt: asOfT}},
	}
	r := scoreWith(snap, Defaults(), asOfT, features.all(), nil, []Action{toBilling{}, autoProlong{}})
	acts := r.Customers[0].Actions
	if len(acts) != 2 || acts[0].ID != "enable_autoprolong" || acts[1].ID != "to_billing" || acts[1].LossDeltaCents != 0 {
		t.Errorf("действия (по эффекту, «к биллингу» последним): %+v", acts)
	}
}

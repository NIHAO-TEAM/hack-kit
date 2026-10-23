package risk

import "testing"

func TestFail(t *testing.T) {
	tx := func(typ, st string) Transaction { return Transaction{Type: typ, Status: st} }
	cases := []struct {
		name   string
		txs    []Transaction
		want   float64
		reason string
	}{
		{"нет продлений", nil, 0, ""},
		{"заказы не считаются", []Transaction{tx("order", "not_performed")}, 0, ""},
		{"2 из 3", []Transaction{tx("prolong", "not_performed"), tx("prolong", "performed"), tx("prolong", "not_performed")}, 2.0 / 3, "2 из 3 продлений не прошли"},
	}
	for _, c := range cases {
		f, reason, ok := fail{}.Eval(ServiceCtx{Transactions: c.txs})
		if !ok || !near(f, c.want) || reason != c.reason {
			t.Errorf("%s: f=%v %q ok=%v", c.name, f, reason, ok)
		}
	}
}

func TestSusp(t *testing.T) {
	if f, _, ok := (susp{}).Eval(ServiceCtx{Service: Service{Status: StatusSuspended}}); !ok || f != 1 {
		t.Errorf("suspended: f=%v ok=%v", f, ok)
	}
	if _, _, ok := (susp{}).Eval(ServiceCtx{Service: Service{Status: StatusActive}}); ok {
		t.Error("active: признак не применим")
	}
}

func TestFlight(t *testing.T) {
	f, reason, ok := flight{}.Eval(ServiceCtx{Account: Account{Balance: 83810, WithdrawBalance: 14592}})
	if !ok || f < 0.174107 || f > 0.174109 || reason == "" {
		t.Errorf("клиент 1: f=%v %q ok=%v", f, reason, ok)
	}
	if f, _, ok := (flight{}).Eval(ServiceCtx{Account: Account{WithdrawBalance: 5}}); !ok || f != 0 {
		t.Errorf("баланс 0: f=%v ok=%v", f, ok)
	}
}

func TestEuros(t *testing.T) {
	for in, want := range map[int64]string{0: "0,00 €", 5: "0,05 €", 144490: "1 444,90 €", 123456789: "1 234 567,89 €"} {
		if got := euros(in); got != want {
			t.Errorf("euros(%d) = %q, ждали %q", in, got, want)
		}
	}
}

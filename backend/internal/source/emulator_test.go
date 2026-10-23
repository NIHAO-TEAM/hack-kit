package source

import (
	"example.com/app/internal/platform/api/gen"
	"testing"
	"time"
)

func emuSnap() Snapshot {
	at := time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC)
	return Snapshot{
		AsOf:     &at,
		Accounts: []Account{{ID: 1, Balance: 10000, TotalReplenished: 10000}, {ID: 2, Balance: 10}},
		Services: []Service{
			{ID: 1, OwnerID: 1, PaymentTerm: "hour", AutoProlong: true, ExpiresAt: at.Add(time.Hour), Status: "active", Price: 100},
			{ID: 2, OwnerID: 1, PaymentTerm: "month", AutoProlong: false, ExpiresAt: at.Add(2 * time.Hour), Status: "active", Price: 500},
			{ID: 3, OwnerID: 2, PaymentTerm: "week", AutoProlong: true, ExpiresAt: at.Add(time.Hour), Status: "active", Price: 500},
			{ID: 4, OwnerID: 1, PaymentTerm: "month", AutoProlong: true, ExpiresAt: at.Add(-5 * 24 * time.Hour), Status: "prolong_wait", Price: 500},
		},
	}
}

func TestAdvance(t *testing.T) {
	src := emuSnap()
	got := Advance(src, 24)
	if want := src.AsOf.Add(24 * time.Hour); !got.AsOf.Equal(want) {
		t.Errorf("asOf %v, ждали %v", got.AsOf, want)
	}
	// почасовая с автопродлением: 24 списания по цене
	if got.Accounts[0].Balance != 10000-24*100 || got.Accounts[0].WithdrawBalance != 24*100 {
		t.Errorf("баланс %d, списано %d", got.Accounts[0].Balance, got.Accounts[0].WithdrawBalance)
	}
	if want := src.Services[0].ExpiresAt.Add(24 * time.Hour); !got.Services[0].ExpiresAt.Equal(want) || got.Services[0].Status != "active" {
		t.Errorf("почасовая: %v %s", got.Services[0].ExpiresAt, got.Services[0].Status)
	}
	cases := []struct {
		i    int
		want string
		why  string
	}{
		{1, "prolong_wait", "срок прошёл без автопродления"},
		{2, "prolong_wait", "автопродление включено, денег нет"},
		{3, "deleted", "prolong_wait старше 5 дней"},
	}
	for _, c := range cases {
		if got.Services[c.i].Status != c.want {
			t.Errorf("%s: статус %s, ждали %s", c.why, got.Services[c.i].Status, c.want)
		}
	}
	if got.Accounts[1].Balance != 10 {
		t.Error("без денег ничего не списывается")
	}
	if src.Accounts[0].Balance != 10000 || src.Services[0].Status != "active" {
		t.Error("исходный снимок изменился")
	}
}

func TestAdvanceDeletesAfterFiveDays(t *testing.T) {
	got := Advance(emuSnap(), 5*24+3)
	if got.Services[1].Status != "deleted" {
		t.Errorf("статус %s", got.Services[1].Status)
	}
}

func TestApplyAction(t *testing.T) {
	s := emuSnap()
	cents := int64(500)
	if err := applyAction(&s, gen2("top_up", 2, &cents)); err != nil || s.Accounts[1].Balance != 510 || s.Accounts[1].TotalReplenished != 500 {
		t.Errorf("top_up: %v %+v", err, s.Accounts[1])
	}
	if err := applyAction(&s, gen2("enable_autoprolong", 1, nil)); err != nil || !s.Services[1].AutoProlong {
		t.Errorf("autoprolong: %v", err)
	}
	for _, a := range []struct {
		id    string
		cust  int64
		cents *int64
	}{{"top_up", 1, nil}, {"top_up", 99, &cents}, {"enable_autoprolong", 99, nil}, {"nope", 1, nil}} {
		if applyAction(&s, gen2(a.id, a.cust, a.cents)) == nil {
			t.Errorf("%s/%d: ждали ошибку", a.id, a.cust)
		}
	}
}

func gen2(id string, cust int64, cents *int64) gen.EmulatorAction {
	return gen.EmulatorAction{ActionId: id, CustomerId: cust, Cents: cents}
}

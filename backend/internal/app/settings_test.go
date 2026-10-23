package app

import (
	"reflect"
	"testing"

	"example.com/app/internal/risk"
)

func lvl(s string) *string { return &s }

func TestDiff(t *testing.T) {
	before := risk.Result{
		Customers: []risk.Customer{
			{ID: 1, Level: lvl("medium")},
			{ID: 2, Level: lvl("low")},
			{ID: 3, Level: lvl("medium")},
			{ID: 4},
		},
		Summary: risk.Summary{ExpectedLossCents: 1000},
	}
	after := risk.Result{
		Customers: []risk.Customer{
			{ID: 1, Level: lvl("high")},
			{ID: 2, Level: lvl("low")},
			{ID: 3, Level: lvl("high")},
			{ID: 4},
		},
		Summary: risk.Summary{ExpectedLossCents: 800},
	}
	d := Diff(before, after, map[int64]string{1: "a@x", 3: "c@x"})
	if want := map[string]int{"medium->high": 2}; !reflect.DeepEqual(d.LevelChanges, want) {
		t.Errorf("LevelChanges = %v, ждали %v", d.LevelChanges, want)
	}
	if d.ExpectedLossDeltaCents != -200 {
		t.Errorf("дельта = %d, ждали -200", d.ExpectedLossDeltaCents)
	}
}

func TestDiffCustomers(t *testing.T) {
	sc := func(v float64) *float64 { return &v }
	var before, after risk.Result
	for i := int64(1); i <= 60; i++ {
		before.Customers = append(before.Customers, risk.Customer{ID: i, Level: lvl("medium"), Score: sc(0.5), ExpectedLossCents: 1000})
		after.Customers = append(after.Customers, risk.Customer{ID: i, Level: lvl("high"), Score: sc(0.7), ExpectedLossCents: 1000 + i*10})
	}
	// без смены уровня — не попадает в список
	before.Customers = append(before.Customers, risk.Customer{ID: 99, Level: lvl("low"), Score: sc(0.1)})
	after.Customers = append(after.Customers, risk.Customer{ID: 99, Level: lvl("low"), Score: sc(0.2)})

	d := Diff(before, after, map[int64]string{60: "big@x"})
	if d.CustomersTotal != 60 || len(d.Customers) != 50 {
		t.Fatalf("total=%d len=%d, ждали 60 и 50", d.CustomersTotal, len(d.Customers))
	}
	first := d.Customers[0]
	if first.ID != 60 || first.Name != "big@x" || first.ExpectedLossDeltaCents != 600 ||
		first.LevelFrom != "medium" || first.LevelTo != "high" || first.ScoreFrom != 0.5 || first.ScoreTo != 0.7 {
		t.Errorf("первый = %+v", first)
	}
	if d.Customers[49].ID != 11 {
		t.Errorf("последний = %d, ждали 11 (сортировка по |Δ| убыв.)", d.Customers[49].ID)
	}
}

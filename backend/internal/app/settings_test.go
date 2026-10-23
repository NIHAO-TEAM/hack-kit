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
	d := Diff(before, after)
	if want := map[string]int{"medium->high": 2}; !reflect.DeepEqual(d.LevelChanges, want) {
		t.Errorf("LevelChanges = %v, ждали %v", d.LevelChanges, want)
	}
	if d.ExpectedLossDeltaCents != -200 {
		t.Errorf("дельта = %d, ждали -200", d.ExpectedLossDeltaCents)
	}
}

package model

import (
	"reflect"
	"testing"

	"example.com/app/internal/audit"
)

func TestDiff(t *testing.T) {
	from := Settings{Weights: map[string]float64{"noauto": 0.55, "susp": 0.1}, High: 70, Medium: 40, HorizonDays: 30, PriceBasis: "a", AsOfMode: AsOfSnapshot}
	to := from.Apply(Patch{Weights: map[string]float64{"noauto": 0.6}})
	want := []audit.Change{{Field: "weights.noauto", From: "0.55", To: "0.6"}}
	if got := Diff(from, to); !reflect.DeepEqual(got, want) {
		t.Errorf("вес: %+v", got)
	}
	if got := Diff(from, from); len(got) != 0 {
		t.Errorf("без правок: %+v", got)
	}
	h, d, m := 80.0, 14, AsOfServer
	to = from.Apply(Patch{High: &h, HorizonDays: &d, AsOfMode: &m})
	if got := Diff(from, to); len(got) != 3 || got[0].Field != "thresholds.high" || got[1].Field != "horizonDays" || got[2].Field != "asOfMode" {
		t.Errorf("поля: %+v", got)
	}
}

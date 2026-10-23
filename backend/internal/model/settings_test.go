package model

import (
	"reflect"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestApplyMergesWeights(t *testing.T) {
	cur := Settings{Weights: map[string]float64{"noauto": 0.55, "pwait": 0.8}, High: 70, Medium: 50, HorizonDays: 30, PriceBasis: "per_period", AsOfMode: AsOfSnapshot}
	next := cur.Apply(Patch{Weights: map[string]float64{"noauto": 0.85}, High: ptr(80.0)})
	if want := (map[string]float64{"noauto": 0.85, "pwait": 0.8}); !reflect.DeepEqual(next.Weights, want) {
		t.Errorf("веса = %v, ждали %v", next.Weights, want)
	}
	if next.High != 80 || next.Medium != 50 || next.HorizonDays != 30 {
		t.Errorf("пороги/горизонт: %+v", next)
	}
	if cur.Weights["noauto"] != 0.55 || cur.High != 70 {
		t.Errorf("исходные настройки изменились: %+v", cur)
	}
}

func TestProblems(t *testing.T) {
	known := []string{"noauto", "pwait"}
	cases := []struct {
		name  string
		s     Settings
		field string
	}{
		{"верные", Settings{Weights: map[string]float64{"noauto": 0.5}, AsOfMode: AsOfServer}, ""},
		{"неизвестный признак", Settings{Weights: map[string]float64{"zzz": 0.5}, AsOfMode: AsOfSnapshot}, "weights.zzz"},
		{"asOfMode", Settings{AsOfMode: "x"}, "asOfMode"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ps := c.s.Problems(known)
			if c.field == "" {
				if len(ps) != 0 {
					t.Fatalf("ждали без ошибок, есть %v", ps)
				}
				return
			}
			if len(ps) != 1 || ps[0].Field != c.field {
				t.Fatalf("ошибки = %v, ждали поле %s", ps, c.field)
			}
		})
	}
}

func TestParseEmpty(t *testing.T) {
	s, err := Parse(nil)
	if err != nil || s.Weights != nil {
		t.Fatalf("Parse(nil) = %+v, %v", s, err)
	}
}

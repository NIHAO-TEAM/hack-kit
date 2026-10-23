package app

import (
	"testing"

	"example.com/app/internal/risk"
)

func TestQualityChecksMapping(t *testing.T) {
	got := qualityChecks([]risk.CheckResult{
		{ID: "a", OK: true, Message: "Нет услуг …"},
		{ID: "b", Message: "2 услуги …", Count: 2, Items: []risk.CheckItem{{ServiceID: 5, AccountID: 7}}},
	})
	if len(got) != 2 || !got[0].Ok || got[0].Items != nil || *got[0].Message != "Нет услуг …" {
		t.Fatalf("прошедшая: %+v", got)
	}
	if got[1].Ok || got[1].Items == nil || (*got[1].Items)[0].ServiceId != 5 || (*got[1].Items)[0].AccountId != 7 {
		t.Errorf("найденная: %+v", got[1])
	}
	var r *Recalc
	if r.QualityChecks(1) != nil {
		t.Error("без воркера проверок нет")
	}
}

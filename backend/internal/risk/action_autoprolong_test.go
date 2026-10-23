package risk

import (
	"testing"
	"time"
)

func autoProlongSnapshot() Snapshot {
	return Snapshot{
		AsOf:     asOfT,
		Accounts: []Account{{ID: 1, Balance: 100000}, {ID: 2, Balance: 100000}},
		Services: []Service{
			// noauto: срок через 3,5 дня → f = 0,5, P = 0,275
			{ID: 10, OwnerID: 1, PaymentTerm: "month", AutoProlong: false, Status: StatusActive,
				ExpiresAt: asOfT.Add(84 * time.Hour), Price: 2000},
			{ID: 11, OwnerID: 1, PaymentTerm: "month", AutoProlong: false, Status: StatusDeleted,
				ExpiresAt: asOfT, Price: 900},
			// срок далеко: noauto применим, но f = 0 — действия нет
			{ID: 20, OwnerID: 2, PaymentTerm: "month", AutoProlong: false, Status: StatusActive,
				ExpiresAt: asOfT.AddDate(0, 0, 20), Price: 700},
		},
	}
}

func TestAutoProlongAction(t *testing.T) {
	snap := autoProlongSnapshot()
	r := scoreWith(snap, Defaults(), asOfT, features.all(), nil, []Action{autoProlong{}})
	c := r.Customers[0]
	if len(c.Actions) != 1 {
		t.Fatalf("действия клиента 1: %+v", c.Actions)
	}
	a := c.Actions[0]
	if a.ID != "enable_autoprolong" || a.Title != "Включить автопродление" {
		t.Errorf("действие: %+v", a)
	}
	// 0,275·2000 = 550 → после автопродления 0
	if a.LossDeltaCents != -550 || *a.ScoreBefore != 27.5 || *a.ScoreAfter != 0 || *a.LevelAfter != LevelLow {
		t.Errorf("эффект: %+v before=%v after=%v", a, *a.ScoreBefore, *a.ScoreAfter)
	}
	if len(r.Customers[1].Actions) != 0 {
		t.Errorf("у клиента 2 срок далеко: %+v", r.Customers[1].Actions)
	}
	if snap.Services[0].AutoProlong {
		t.Error("действие изменило исходный снимок")
	}
}

func TestAutoProlongApplyTouchesOnlyLiveServicesOfAccount(t *testing.T) {
	out := autoProlong{}.Apply(autoProlongSnapshot(), 1)
	if !out.Services[0].AutoProlong || out.Services[1].AutoProlong || out.Services[2].AutoProlong {
		t.Errorf("автопродление: %+v", out.Services)
	}
}

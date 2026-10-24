package risk

import "testing"

func TestAllocateLossSumsToLoss(t *testing.T) {
	cases := []struct {
		loss int64
		hits []FeatureHit
	}{
		{1000, []FeatureHit{{ID: "funds", Contribution: 0.3}, {ID: "noauto", Contribution: 0.2}, {ID: "fail", Contribution: 0.1}}},
		{1, []FeatureHit{{ID: "a", Contribution: 0.5}, {ID: "b", Contribution: 0.5}}},
		{7, []FeatureHit{{ID: "a", Contribution: 1.0 / 3}, {ID: "b", Contribution: 1.0 / 3}, {ID: "c", Contribution: 1.0 / 3}}},
		{99999, []FeatureHit{{ID: "x", Contribution: 0.77}}},
	}
	for _, c := range cases {
		got := AllocateLoss(c.loss, c.hits)
		var sum int64
		for _, v := range got {
			if v < 0 {
				t.Errorf("отрицательная доля %v", got)
			}
			sum += v
		}
		if sum != c.loss {
			t.Errorf("loss %d: сумма долей %d (%v)", c.loss, sum, got)
		}
	}
}

func TestAllocateLossRemainderToLargest(t *testing.T) {
	// 7 центов на три равные доли: по 2, остаток 1 — первому по ID
	got := AllocateLoss(7, []FeatureHit{{ID: "a", Contribution: 0.2}, {ID: "b", Contribution: 0.2}, {ID: "c", Contribution: 0.2}})
	if got[0] != 3 || got[1] != 2 || got[2] != 2 {
		t.Errorf("получено %v, ждали [3 2 2]", got)
	}
}

func TestAllocateLossEmpty(t *testing.T) {
	if AllocateLoss(0, []FeatureHit{{ID: "a", Contribution: 1}}) != nil || AllocateLoss(100, nil) != nil {
		t.Error("без потери или вкладов — nil")
	}
}

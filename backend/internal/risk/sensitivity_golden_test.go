package risk

import "testing"

type goldenShift struct {
	LevelChanges    int   `json:"levelChanges"`
	TopThreeChanged bool  `json:"topThreeChanged"`
	LossDeltaCents  int64 `json:"lossDeltaCents"`
}

type goldenSensitivity struct {
	AsOf      string `json:"asOf"`
	Source    string `json:"source"`
	Customers int    `json:"customers"`
	Sampled   bool   `json:"sampled"`
	Features  []struct {
		ID     string      `json:"id"`
		Weight float64     `json:"weight"`
		Down   goldenShift `json:"down"`
		Up     goldenShift `json:"up"`
	} `json:"features"`
}

// Чувствительность к весам совпадает с оракулом (testdata/golden/sensitivity): число сменивших уровень,
// пометка про первую тройку и сдвиг потери — до единицы и до цента.
func TestSensitivityGolden(t *testing.T) {
	var g goldenSensitivity
	readJSON(t, "../../../testdata/golden/sensitivity/organizers.json", &g)
	var snap Snapshot
	readJSON(t, "../../../"+g.Source, &snap)
	got := Sensitivity(snap, Defaults(), ts(g.AsOf))
	if got.Sampled != g.Sampled || got.Customers != g.Customers {
		t.Fatalf("sampled/customers: %v/%d, эталон %v/%d", got.Sampled, got.Customers, g.Sampled, g.Customers)
	}
	if len(got.Features) != len(g.Features) {
		t.Fatalf("признаков %d, эталон %d", len(got.Features), len(g.Features))
	}
	for i, w := range g.Features {
		f := got.Features[i]
		if f.ID != w.ID || f.Weight != w.Weight {
			t.Errorf("признак %d: %s/%v, эталон %s/%v", i, f.ID, f.Weight, w.ID, w.Weight)
			continue
		}
		for name, p := range map[string]struct {
			got  Shift
			want goldenShift
		}{"down": {f.Down, w.Down}, "up": {f.Up, w.Up}} {
			if p.got.LevelChanges != p.want.LevelChanges || p.got.TopThreeChanged != p.want.TopThreeChanged || p.got.LossDeltaCents != p.want.LossDeltaCents {
				t.Errorf("%s %s: %+v, эталон %+v", w.ID, name, p.got, p.want)
			}
		}
	}
}

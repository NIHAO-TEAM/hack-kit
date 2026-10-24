package risk

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"
)

type goldenFile struct {
	AsOf     string `json:"asOf"`
	Settings struct {
		High         float64            `json:"high"`
		Medium       float64            `json:"medium"`
		HorizonHours int                `json:"horizonHours"`
		PriceBasis   string             `json:"priceBasis"`
		Weights      map[string]float64 `json:"weights"`
	} `json:"settings"`
	Customers []struct {
		ID                int64                         `json:"id"`
		State             string                        `json:"state"`
		Score             *float64                      `json:"score"`
		Level             *string                       `json:"level"`
		ExpectedLossCents int64                         `json:"expectedLossCents"`
		ExposureCents     int64                         `json:"exposureCents"`
		LostCents         int64                         `json:"lostCents"`
		Features          map[string]map[string]float64 `json:"features"`
		LossByFeature     map[string]int64              `json:"lossByFeature"`
	} `json:"customers"`
	Summary struct {
		Levels            map[string]int `json:"levels"`
		ExpectedLossCents int64          `json:"expectedLossCents"`
		ExposureCents     int64          `json:"exposureCents"`
		LostCents         int64          `json:"lostCents"`
		LossByFeature     []struct {
			ID        string `json:"id"`
			LossCents int64  `json:"lossCents"`
			Customers int    `json:"customers"`
		} `json:"lossByFeature"`
	} `json:"summary"`
}

// Модель совпадает с эталоном оракула (testdata/golden): скор, уровень, состояние, f признаков,
// деньги клиента и сводка. Поле actions не сверяется — его проверяет money-actions.
func TestGolden(t *testing.T) {
	cases := []struct{ golden, snapshot string }{
		{"organizers.json", "../../../cases/analizator/data.json"},
		{"stand.json", "../../../testdata/golden/stand.snapshot.json"},
		{"edge-empty.json", "../../../testdata/golden/empty.snapshot.json"},
		{"edge-fresh-prolong-wait.json", "../../../testdata/golden/fresh-prolong-wait.snapshot.json"},
		{"edge-gone-only.json", "../../../testdata/golden/gone-only.snapshot.json"},
		{"edge-no-services.json", "../../../testdata/golden/no-services.snapshot.json"},
	}
	for _, c := range cases {
		t.Run(c.golden, func(t *testing.T) {
			var g goldenFile
			readJSON(t, "../../../testdata/golden/"+c.golden, &g)
			var snap Snapshot
			readJSON(t, c.snapshot, &snap)
			asOf := ts(g.AsOf)
			st := Defaults()
			st.High, st.Medium, st.PriceBasis = g.Settings.High, g.Settings.Medium, g.Settings.PriceBasis
			st.HorizonDays = g.Settings.HorizonHours / 24
			st.Weights = g.Settings.Weights

			res := Score(snap, st, asOf)
			if len(res.Customers) != len(g.Customers) {
				t.Fatalf("клиентов %d, в эталоне %d", len(res.Customers), len(g.Customers))
			}
			for i, want := range g.Customers {
				got := res.Customers[i]
				if got.ID != want.ID || got.State != want.State {
					t.Fatalf("клиент %d: id %d состояние %s, эталон %d %s", i, got.ID, got.State, want.ID, want.State)
				}
				if (got.Score == nil) != (want.Score == nil) || (got.Score != nil && math.Abs(*got.Score-*want.Score) > 0.01) {
					t.Errorf("клиент %d: скор %v, эталон %v", got.ID, ptr(got.Score), ptr(want.Score))
				}
				if (got.Level == nil) != (want.Level == nil) || (got.Level != nil && *got.Level != *want.Level) {
					t.Errorf("клиент %d: уровень не совпал", got.ID)
				}
				if got.ExpectedLossCents != want.ExpectedLossCents || got.ExposureCents != want.ExposureCents || got.LostCents != want.LostCents {
					t.Errorf("клиент %d: деньги %d/%d/%d, эталон %d/%d/%d", got.ID,
						got.ExpectedLossCents, got.ExposureCents, got.LostCents,
						want.ExpectedLossCents, want.ExposureCents, want.LostCents)
				}
				gotLoss := map[string]int64{}
				for _, sr := range got.Services {
					for i, v := range AllocateLoss(sr.LossCents, sr.Features) {
						if v != 0 {
							gotLoss[sr.Features[i].ID] += v
						}
					}
				}
				if len(gotLoss) != len(want.LossByFeature) {
					t.Errorf("клиент %d: потеря по причинам %v, эталон %v", got.ID, gotLoss, want.LossByFeature)
				}
				for k, v := range want.LossByFeature {
					if gotLoss[k] != v {
						t.Errorf("клиент %d: потеря по причине %s = %d, эталон %d", got.ID, k, gotLoss[k], v)
					}
				}
				if len(got.Services) != len(want.Features) {
					t.Errorf("клиент %d: услуг %d, в эталоне %d", got.ID, len(got.Services), len(want.Features))
				}
				for _, sr := range got.Services {
					wf := want.Features[strconv.FormatInt(sr.ID, 10)]
					if len(sr.Features) != len(wf) {
						t.Errorf("услуга %d: признаков %d, в эталоне %d", sr.ID, len(sr.Features), len(wf))
					}
					for _, h := range sr.Features {
						w, ok := wf[h.ID]
						if !ok || math.Abs(h.F-w) > 1e-6 {
							t.Errorf("услуга %d, %s: %v, эталон %v (есть=%v)", sr.ID, h.ID, h.F, w, ok)
						}
					}
				}
			}
			sum := res.Summary
			if sum.ExpectedLossCents != g.Summary.ExpectedLossCents || sum.ExposureCents != g.Summary.ExposureCents ||
				sum.LostCents != g.Summary.LostCents {
				t.Errorf("сводка: деньги %d/%d/%d, эталон %d/%d/%d", sum.ExpectedLossCents, sum.ExposureCents, sum.LostCents,
					g.Summary.ExpectedLossCents, g.Summary.ExposureCents, g.Summary.LostCents)
			}
			if len(sum.LossByFeature) != len(g.Summary.LossByFeature) {
				t.Fatalf("сводка: причин %d, в эталоне %d", len(sum.LossByFeature), len(g.Summary.LossByFeature))
			}
			var byFeature int64
			for i, want := range g.Summary.LossByFeature {
				got := sum.LossByFeature[i]
				if got.ID != want.ID || got.LossCents != want.LossCents || got.Customers != want.Customers {
					t.Errorf("сводка: причина %d = %+v, эталон %+v", i, got, want)
				}
				byFeature += got.LossCents
			}
			if byFeature != sum.ExpectedLossCents {
				t.Errorf("сводка: сумма по причинам %d, потеря %d", byFeature, sum.ExpectedLossCents)
			}
			for k, v := range g.Summary.Levels {
				if sum.Levels[k] != v {
					t.Errorf("сводка: уровень %s = %d, эталон %d", k, sum.Levels[k], v)
				}
			}
		})
	}
}

func ptr(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}

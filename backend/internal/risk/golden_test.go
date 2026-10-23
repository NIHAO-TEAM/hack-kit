package risk

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
	"testing"
)

// Признаки pwait и noauto на данных организаторов совпадают с эталоном оракула (testdata/golden).
// Полная сверка на всех числах — в risk-features, когда зарегистрированы остальные признаки.
func TestFeaturesMatchOracleGolden(t *testing.T) {
	var snap Snapshot
	readJSON(t, "../../../cases/analizator/data.json", &snap)
	var g struct {
		AsOf      string `json:"asOf"`
		Customers []struct {
			Features map[string]map[string]float64 `json:"features"`
		} `json:"customers"`
	}
	readJSON(t, "../../../testdata/golden/organizers.json", &g)
	asOf := ts(g.AsOf)

	byID := map[int64]Service{}
	for _, s := range snap.Services {
		byID[s.ID] = s
	}
	checked := 0
	for _, c := range g.Customers {
		for sid, feats := range c.Features {
			id, _ := strconv.ParseInt(sid, 10, 64)
			sv := byID[id]
			for _, f := range []Feature{pwait{}, noauto{}} {
				got, _, ok := f.Eval(ServiceCtx{AsOf: asOf, Service: sv})
				want, has := feats[f.ID()]
				if ok != has {
					t.Fatalf("услуга %d, %s: применим=%v, в эталоне=%v", id, f.ID(), ok, has)
				}
				if ok && math.Abs(got-want) > 5e-7 {
					t.Errorf("услуга %d, %s: %v, эталон %v", id, f.ID(), got, want)
				}
				checked++
			}
		}
	}
	if checked == 0 {
		t.Fatal("в эталоне нет услуг")
	}
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

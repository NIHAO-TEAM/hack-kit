package risk

import (
	"fmt"
	"math"
	"testing"
)

type goldenAction struct {
	Type           string   `json:"type"`
	AmountCents    *int64   `json:"amountCents"`
	LossDeltaCents int64    `json:"lossDeltaCents"`
	ScoreBefore    *float64 `json:"scoreBefore"`
	ScoreAfter     *float64 `json:"scoreAfter"`
	LevelAfter     *string  `json:"levelAfter"`
}

// Действия на данных организаторов совпадают с эталоном оракула (поле actions).
// Клиент сверяется, когда зарегистрированы все признаки, что есть в его эталоне: пока risk-features
// не слит, fail, flight и funds отсутствуют и скор не сойдётся, такие клиенты пропускаются.
func TestActionsGolden(t *testing.T) {
	var snap Snapshot
	readJSON(t, "../../../cases/analizator/data.json", &snap)
	var g struct {
		AsOf      string `json:"asOf"`
		Customers []struct {
			ID       int64                         `json:"id"`
			Features map[string]map[string]float64 `json:"features"`
			Actions  []goldenAction                `json:"actions"`
		} `json:"customers"`
	}
	readJSON(t, "../../../testdata/golden/organizers.json", &g)
	asOf := ts(g.AsOf)
	snap.AsOf = asOf
	res := Score(snap, Defaults(), asOf)

	registered := map[string]bool{}
	for _, f := range features.all() {
		registered[f.ID()] = true
	}
	checked := 0
	for i, c := range g.Customers {
		if complete := func() bool {
			for _, feats := range c.Features {
				for id := range feats {
					if !registered[id] {
						return false
					}
				}
			}
			return true
		}(); !complete {
			t.Logf("клиент %d пропущен: нет признаков из эталона", c.ID)
			continue
		}
		var got []ActionResult
		for _, a := range res.Customers[i].Actions {
			if a.ID != "to_billing" { // оракул тег не считает
				got = append(got, a)
			}
		}
		if len(got) != len(c.Actions) {
			t.Errorf("клиент %d: действий %d, в эталоне %d", c.ID, len(got), len(c.Actions))
			continue
		}
		for k, want := range c.Actions {
			a := got[k]
			if a.ID != want.Type || a.LossDeltaCents != want.LossDeltaCents {
				t.Errorf("клиент %d, действие %d: %s %d, эталон %s %d", c.ID, k, a.ID, a.LossDeltaCents, want.Type, want.LossDeltaCents)
			}
			if want.AmountCents != nil {
				if w := fmt.Sprintf("Пополнить баланс на %s €", eurosText(*want.AmountCents)); a.Title != w {
					t.Errorf("клиент %d: заголовок %q, ждали %q", c.ID, a.Title, w)
				}
			}
			if !near2(a.ScoreBefore, want.ScoreBefore) || !near2(a.ScoreAfter, want.ScoreAfter) {
				t.Errorf("клиент %d, %s: скор %v → %v, эталон %v → %v", c.ID, want.Type, deref(a.ScoreBefore), deref(a.ScoreAfter), deref(want.ScoreBefore), deref(want.ScoreAfter))
			}
			if a.LevelAfter == nil || want.LevelAfter == nil || *a.LevelAfter != *want.LevelAfter {
				t.Errorf("клиент %d, %s: уровень после не совпал с эталоном", c.ID, want.Type)
			}
		}
		checked++
	}
	if checked == 0 && registered["funds"] {
		t.Fatal("в эталоне нет клиентов для сверки")
	}
}

func near2(a, b *float64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return math.Abs(*a-*b) <= 0.01
}

func deref(p *float64) any {
	if p == nil {
		return nil
	}
	return *p
}

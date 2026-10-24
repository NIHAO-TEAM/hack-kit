package risk

import "testing"

// Клиент 9 на данных организаторов: пополнение с ценой и сочетание двух действий; клиент 1: действие без цены.
func TestActionCompareOrganizers(t *testing.T) {
	var snap Snapshot
	readJSON(t, "../../../cases/analizator/data.json", &snap)
	var g struct {
		AsOf string `json:"asOf"`
	}
	readJSON(t, "../../../testdata/golden/organizers.json", &g)
	asOf := ts(g.AsOf)
	snap.AsOf = asOf
	res := Score(snap, Defaults(), asOf)
	byID := map[int64]Customer{}
	for _, c := range res.Customers {
		byID[c.ID] = c
	}
	if len(features.all()) == 0 {
		t.Skip("нет признаков")
	}

	c9 := byID[9]
	var top *ActionResult
	for i := range c9.Actions {
		if c9.Actions[i].ID == "top_up" {
			top = &c9.Actions[i]
		}
	}
	if top == nil {
		t.Fatal("у клиента 9 нет пополнения")
	}
	if top.CostCents != 52984 || top.LossDeltaCents != -102749 {
		t.Errorf("пополнение: цена %d, эффект %d", top.CostCents, top.LossDeltaCents)
	}
	if top.ReturnPerEuro == nil || *top.ReturnPerEuro != 1.94 {
		t.Errorf("на каждый €: %v, ждали 1,94", deref(top.ReturnPerEuro))
	}
	if c9.ActionComboCents == nil {
		t.Fatal("у клиента 9 нет сочетания")
	}
	if *c9.ActionComboCents >= top.LossDeltaCents {
		t.Errorf("сочетание %d не лучше пополнения %d", *c9.ActionComboCents, top.LossDeltaCents)
	}

	c1 := byID[1]
	if len(c1.Actions) != 1 || c1.Actions[0].ID != "enable_autoprolong" {
		t.Fatalf("действия клиента 1: %+v", c1.Actions)
	}
	if a := c1.Actions[0]; a.CostCents != 0 || a.ReturnPerEuro != nil || a.LossDeltaCents != -37508 {
		t.Errorf("автопродление: %+v", a)
	}
	if c1.ActionComboCents != nil {
		t.Error("у клиента 1 одно действие, сочетания быть не должно")
	}
}

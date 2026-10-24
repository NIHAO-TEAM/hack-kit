package risk

import (
	"testing"
	"time"
)

func TestShiftWeightKeepsOriginalAndClamps(t *testing.T) {
	st := Defaults()
	st.Weights["noauto"] = 0.9
	up := ShiftWeight(st, "noauto", 1+SensitivityShift)
	if up.Weights["noauto"] != 1 {
		t.Fatalf("вес выше 1 не ограничен: %v", up.Weights["noauto"])
	}
	down := ShiftWeight(st, "noauto", 1-SensitivityShift)
	if got := down.Weights["noauto"]; got < 0.719 || got > 0.721 {
		t.Fatalf("−20 %% от 0,9 = 0,72, получили %v", got)
	}
	if st.Weights["noauto"] != 0.9 {
		t.Fatalf("исходные настройки изменены: %v", st.Weights["noauto"])
	}
}

func TestSensitivityOrganizers(t *testing.T) {
	var snap Snapshot
	readJSON(t, "../../../cases/analizator/data.json", &snap)
	asOf := ts("2026-10-19T15:00:00Z")
	st := Defaults()
	res := Sensitivity(snap, st, asOf)
	if res.Sampled || res.Customers != len(snap.Accounts) {
		t.Fatalf("выборка не нужна: sampled=%v customers=%d", res.Sampled, res.Customers)
	}
	if len(res.Features) == 0 {
		t.Fatal("нет признаков")
	}
	for i := 1; i < len(res.Features); i++ {
		if res.Features[i-1].ID >= res.Features[i].ID {
			t.Fatalf("признаки не по id: %s, %s", res.Features[i-1].ID, res.Features[i].ID)
		}
	}
	for _, f := range res.Features {
		if f.Weight == 0 {
			t.Fatalf("признак %s с нулевым весом в ответе", f.ID)
		}
		// вес вниз не увеличивает потерю, вверх не уменьшает
		if f.Down.LossDeltaCents > 0 || f.Up.LossDeltaCents < 0 {
			t.Errorf("%s: потеря идёт против веса: down=%d up=%d", f.ID, f.Down.LossDeltaCents, f.Up.LossDeltaCents)
		}
	}
	if again := Sensitivity(snap, st, asOf); len(again.Features) != len(res.Features) || again.Features[0] != res.Features[0] {
		t.Fatal("расчёт не детерминирован")
	}
}

func TestSensitivityZeroWeightSkipped(t *testing.T) {
	var snap Snapshot
	readJSON(t, "../../../cases/analizator/data.json", &snap)
	st := Defaults()
	st.Weights["noauto"] = 0
	for _, f := range Sensitivity(snap, st, ts("2026-10-19T15:00:00Z")).Features {
		if f.ID == "noauto" {
			t.Fatal("признак с нулевым весом не должен считаться")
		}
	}
}

func TestSampleSnapshot(t *testing.T) {
	n := SensitivityMaxCustomers + 10
	s := Snapshot{AsOf: time.Unix(0, 0)}
	for i := 1; i <= n; i++ {
		s.Accounts = append(s.Accounts, Account{ID: int64(i)})
		s.Services = append(s.Services, Service{ID: int64(i), OwnerID: int64(i)})
		s.Transactions = append(s.Transactions, Transaction{ID: int64(i), ServiceID: int64(i)})
	}
	small, sampled := sampleSnapshot(Snapshot{Accounts: s.Accounts[:SensitivityMaxCustomers]})
	if sampled || len(small.Accounts) != SensitivityMaxCustomers {
		t.Fatal("ровно 20 000 клиентов — без выборки")
	}
	got, sampled := sampleSnapshot(s)
	if !sampled || len(got.Accounts) != SensitivitySampleSize || len(got.Services) != SensitivitySampleSize || len(got.Transactions) != SensitivitySampleSize {
		t.Fatalf("выборка: sampled=%v %d/%d/%d", sampled, len(got.Accounts), len(got.Services), len(got.Transactions))
	}
	// порядок входа не влияет на выборку
	rev := Snapshot{AsOf: s.AsOf}
	for i := len(s.Accounts) - 1; i >= 0; i-- {
		rev.Accounts = append(rev.Accounts, s.Accounts[i])
	}
	again, _ := sampleSnapshot(rev)
	in := map[int64]bool{}
	for _, a := range got.Accounts {
		in[a.ID] = true
	}
	for _, a := range again.Accounts {
		if !in[a.ID] {
			t.Fatalf("выборка зависит от порядка: %d", a.ID)
		}
	}
}

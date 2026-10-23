package source

import (
	"bytes"
	"os"
	"reflect"
	"testing"
)

// Набор стенда проходит те же проверки, что и любой снимок.
func TestStandSeedParses(t *testing.T) {
	s := parseFile(t, standSeed)
	if len(s.Accounts) != 6 || s.Accounts[0].ID != 1001 {
		t.Fatalf("аккаунтов %d, первый id %d", len(s.Accounts), s.Accounts[0].ID)
	}
}

// Склейка даёт 16 аккаунтов; данные организаторов не меняются.
func TestMergeStand(t *testing.T) {
	body, err := mergeSeeds(organizers, standSeed)
	if err != nil {
		t.Fatal(err)
	}
	merged := parseFile(t, body)
	base := parseFile(t, organizers)
	if len(merged.Accounts) != 16 || len(merged.Services) != len(base.Services)+5 || len(merged.Transactions) != len(base.Transactions)+5 {
		t.Fatalf("счётчики %d/%d/%d", len(merged.Accounts), len(merged.Services), len(merged.Transactions))
	}
	for i, a := range base.Accounts {
		if merged.Accounts[i] != a {
			t.Errorf("аккаунт %d организаторов изменился", a.ID)
		}
	}
}

func TestSeedBodySwitch(t *testing.T) {
	t.Setenv("SEED_STAND", "")
	if b, _ := seedBody(); !bytes.Equal(b, organizers) {
		t.Error("без SEED_STAND сид должен быть набором организаторов")
	}
	t.Setenv("SEED_STAND", "true")
	b, err := seedBody()
	if err != nil || bytes.Equal(b, organizers) {
		t.Errorf("с SEED_STAND сид должен включать стенд: %v", err)
	}
}

// Снимок для golden стенда совпадает со склейкой, которую делает сервер.
func TestStandGoldenSnapshotMatchesMerge(t *testing.T) {
	body, err := mergeSeeds(organizers, standSeed)
	if err != nil {
		t.Fatal(err)
	}
	want := parseFile(t, body)
	got := parseFile(t, mustRead(t, "../../../testdata/golden/stand.snapshot.json"))
	if !reflect.DeepEqual(got.Accounts, want.Accounts) || !reflect.DeepEqual(got.Services, want.Services) ||
		!reflect.DeepEqual(got.Transactions, want.Transactions) {
		t.Error("testdata/golden/stand.snapshot.json расходится со склейкой organizers.json + stand.json")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

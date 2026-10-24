package source

import (
	"errors"
	"slices"
	"testing"
	"time"
)

func TestIngestTouchedAccounts(t *testing.T) {
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	base := Snapshot{
		Accounts: []Account{{ID: 1}, {ID: 2}, {ID: 3}},
		Services: []Service{{ID: 10, OwnerID: 1, CreatedAt: at}, {ID: 20, OwnerID: 2, CreatedAt: at}, {ID: 30, OwnerID: 3, CreatedAt: at}},
	}
	g := &eventWriter{state: newIngestState(&base)}
	bal, st := int64(5), "suspended"
	items := []ingestItem{
		{ev: StreamEvent{AccountID: 3, Balance: &bal}},
		{ev: StreamEvent{AccountID: 1, ServiceID: 10, Status: &st}},
		{ev: StreamEvent{AccountID: 3, Balance: &bal}},              // повтор клиента
		{ev: StreamEvent{AccountID: 2, ServiceID: 20, Status: &st}}, // строку не затронуло
	}
	skipped := map[int]error{3: errors.New("нет строки")}
	got := g.touched(items, []int{0, 1, 2, 3}, skipped)
	if want := []int64{1, 3}; !slices.Equal(got, want) {
		t.Fatalf("клиенты %v, ждали %v", got, want)
	}
	if got := g.touched(items, nil, nil); len(got) != 0 {
		t.Fatalf("без записанных событий клиентов нет, получили %v", got)
	}
}

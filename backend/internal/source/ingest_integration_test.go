//go:build integration

package source_test

import (
	"context"
	"reflect"
	"sort"
	"testing"

	"example.com/app/internal/apitest"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/source"
)

// seeded поднимает стенд с сидом и возвращает его снимок и версию.
func seeded(t *testing.T) (*apitest.Stand, source.Snapshot, int64) {
	t.Helper()
	s := apitest.New(t)
	if err := source.OnStart(context.Background(), s.Pool, events.New(), s.Clock, true); err != nil {
		t.Fatal(err)
	}
	snap, v, err := source.NewReader(s.Pool).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return s, snap, v
}

func sequence(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

// normalize приводит снимок к виду, пригодному для сравнения: порядок по номеру, время в UTC.
func normalize(s *source.Snapshot) {
	sort.Slice(s.Accounts, func(i, j int) bool { return s.Accounts[i].ID < s.Accounts[j].ID })
	sort.Slice(s.Services, func(i, j int) bool { return s.Services[i].ID < s.Services[j].ID })
	for i := range s.Services {
		s.Services[i].CreatedAt = s.Services[i].CreatedAt.UTC()
		s.Services[i].ExpiresAt = s.Services[i].ExpiresAt.UTC()
	}
}

// 2.1: после потока строки в базе такие же, как если бы события применили к снимку целиком.
func TestIngestRowsMatchFullSnapshot(t *testing.T) {
	s, snap, _ := seeded(t)
	evs := source.GenerateStream(snap, 2500, 1)

	want := snap
	want.Accounts = append([]source.Account(nil), snap.Accounts...)
	want.Services = append([]source.Service(nil), snap.Services...)
	for i, ev := range evs {
		if err := source.ApplyStreamEvent(&want, ev); err != nil {
			t.Fatalf("эталон, событие %d: %v", i, err)
		}
	}

	run := source.RunIngest(context.Background(), s.Pool, snap, sequence(len(evs)), evs)
	for _, o := range run.Outcomes {
		if o.Err != nil {
			t.Fatalf("событие %d: %v", o.Seq, o.Err)
		}
	}
	if len(run.Outcomes) != len(evs) {
		t.Fatalf("итогов %d, событий %d", len(run.Outcomes), len(evs))
	}
	got, _, err := source.NewReader(s.Pool).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	normalize(&got)
	normalize(&want)
	if !reflect.DeepEqual(got.Accounts, want.Accounts) {
		t.Error("счета в базе не совпали с эталоном")
	}
	if !reflect.DeepEqual(got.Services, want.Services) {
		t.Error("услуги в базе не совпали с эталоном")
	}
	if len(got.Transactions) != len(snap.Transactions) {
		t.Errorf("платежей %d, было %d: поток их не создаёт", len(got.Transactions), len(snap.Transactions))
	}
}

// 2.2: события идут пачками, версия растёт на пачку, о каждой пачке одно объявление.
func TestIngestBatchesAndVersions(t *testing.T) {
	s, snap, before := seeded(t)
	evs := source.GenerateStream(snap, 1000, 2)
	run := source.RunIngest(context.Background(), s.Pool, snap, sequence(len(evs)), evs)

	batches := len(run.BatchSizes)
	if batches == 0 || batches > 100 {
		t.Errorf("пачек %d на 1000 событий, ждали от 1 до 100: %v", batches, run.BatchSizes)
	}
	total := 0
	for _, n := range run.BatchSizes {
		if n < 1 || n > 1000 {
			t.Errorf("в пачке %d событий", n)
		}
		total += n
	}
	if total != len(evs) {
		t.Errorf("в пачках %d событий из %d", total, len(evs))
	}
	_, after, err := source.NewReader(s.Pool).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if after-before != int64(batches) {
		t.Errorf("версия выросла на %d, пачек %d", after-before, batches)
	}
	if len(run.Published) != batches || (batches > 0 && run.Published[batches-1] != after) {
		t.Errorf("объявлено %v, пачек %d, последняя версия %d", run.Published, batches, after)
	}
}

// 2.3: плохое событие уходит в отказ, остальные пачки применяются; повтор номера не применяется.
func TestIngestFailedEventAndDuplicate(t *testing.T) {
	s, snap, _ := seeded(t)
	evs := source.GenerateStream(snap, 10, 3)
	acc := evs[0].AccountID
	neg, huge := int64(-1), int64(2_000_000_000)
	st := "bogus"
	bad := []source.StreamEvent{
		{Kind: source.EventTopUp, AccountID: 999999, Balance: &huge}, // нет такого счёта
		{Kind: source.EventTopUp, AccountID: acc, Balance: &neg},     // отрицательная сумма
		{Kind: source.EventServiceStop, ServiceID: 999999, Status: &st},
	}
	all := append(append([]source.StreamEvent{}, evs[:5]...), bad...)
	all = append(all, evs[5:]...)
	seqs := sequence(len(all))
	// повтор номера 0: второй раз не применяется и не считается событием
	all = append(all, evs[0])
	seqs = append(seqs, 0)

	run := source.RunIngest(context.Background(), s.Pool, snap, seqs, all)
	if run.Duplicates != 1 {
		t.Errorf("повторов %d, ждали 1", run.Duplicates)
	}
	failed, ok := 0, 0
	for _, o := range run.Outcomes {
		if o.Err != nil {
			failed++
			if o.Seq < 5 || o.Seq > 7 {
				t.Errorf("отказ у события %d: %v", o.Seq, o.Err)
			}
		} else {
			ok++
		}
	}
	if failed != 3 || ok != len(evs) {
		t.Errorf("отказов %d, применено %d; ждали 3 и %d", failed, ok, len(evs))
	}
	if len(run.Outcomes) != len(all)-1 {
		t.Errorf("итогов %d, ждали %d", len(run.Outcomes), len(all)-1)
	}
}

// 3.1 на CI-размере: 10 000 клиентов, 10 000 событий через весь API, согласованность 0 центов.
func TestStreamBigSnapshotConsistent(t *testing.T) {
	if testing.Short() {
		t.Skip("долгий")
	}
	runBench(t, 10000, 10000)
}

// 2.6: synchronous_commit off стоит только в транзакции пачки, на остальных соединениях — по умолчанию.
func TestBatchTxSynchronousCommitOffOnlyInBatch(t *testing.T) {
	s := apitest.New(t)
	inside, outside, err := source.BatchTxSyncCommit(context.Background(), s.Pool)
	if err != nil {
		t.Fatal(err)
	}
	if inside != "off" {
		t.Errorf("в транзакции пачки synchronous_commit = %q, ждали off", inside)
	}
	if outside != "on" {
		t.Errorf("вне пачки synchronous_commit = %q, ждали on", outside)
	}
}

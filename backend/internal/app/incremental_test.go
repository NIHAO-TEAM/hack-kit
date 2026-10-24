package app

import (
	"context"
	"math/rand"
	"reflect"
	"slices"
	"testing"
	"time"

	"example.com/app/internal/risk"
)

// touchRows возвращает снимок только из строк клиентов ids: счета, их услуги и платежи этих услуг.
func touchRows(s risk.Snapshot, ids []int64) risk.Snapshot {
	want := map[int64]bool{}
	for _, id := range ids {
		want[id] = true
	}
	sub := risk.Snapshot{AsOf: s.AsOf}
	owned := map[int64]bool{}
	for _, a := range s.Accounts {
		if want[a.ID] {
			sub.Accounts = append(sub.Accounts, a)
		}
	}
	for _, sv := range s.Services {
		if want[sv.OwnerID] {
			sub.Services = append(sub.Services, sv)
			owned[sv.ID] = true
		}
	}
	for _, t := range s.Transactions {
		if owned[t.ServiceID] {
			sub.Transactions = append(sub.Transactions, t)
		}
	}
	return sub
}

// mutate делает событие потока по случайному клиенту: правка баланса или услуги. Возвращает клиента.
func mutate(rng *rand.Rand, s *risk.Snapshot) int64 {
	if rng.Intn(3) == 0 {
		a := &s.Accounts[rng.Intn(len(s.Accounts))]
		a.Balance = int64(rng.Intn(50_000))
		return a.ID
	}
	sv := &s.Services[rng.Intn(len(s.Services))]
	switch rng.Intn(3) {
	case 0:
		sv.Status = []string{risk.StatusActive, risk.StatusSuspended, risk.StatusProlongWait, risk.StatusBlocked, risk.StatusDeleted}[rng.Intn(5)]
	case 1:
		sv.AutoProlong = !sv.AutoProlong
	default:
		sv.ExpiresAt = s.AsOf.Add(time.Duration(rng.Intn(60*24)-10*24) * time.Hour)
		if sv.ExpiresAt.Before(sv.CreatedAt) {
			sv.ExpiresAt = sv.CreatedAt
		}
	}
	return sv.OwnerID
}

// 2.2: частичный пересчёт даёт тот же итог, что полный, до цента: сводка, проверки, причины, клиенты.
func TestRescoreEqualsFull(t *testing.T) {
	rs := benchSnapshot(t, 1500)
	st := risk.Defaults()
	rng := rand.New(rand.NewSource(7))
	prev := risk.Score(rs, st, rs.AsOf)

	events, partial, fallback := 0, 0, 0
	for events < 1000 {
		n := 1 + rng.Intn(25)
		var ids []int64
		for i := 0; i < n; i++ {
			ids = append(ids, mutate(rng, &rs))
		}
		events += n
		want := risk.Score(rs, st, rs.AsOf)
		got, err := risk.Rescore(context.Background(), prev, touchRows(rs, ids), ids, st, risk.Options{})
		switch err {
		case nil:
			partial++
		case risk.ErrNeedFull:
			fallback++
			got = want
		default:
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			for i := range want.Customers {
				if !reflect.DeepEqual(got.Customers[i], want.Customers[i]) {
					t.Fatalf("после %d событий клиент %d: частичный %+v, полный %+v", events, want.Customers[i].ID, got.Customers[i], want.Customers[i])
				}
			}
			for i := range want.Checks {
				if !reflect.DeepEqual(got.Checks[i], want.Checks[i]) {
					t.Fatalf("после %d событий проверка %s: частичная %v, полная %v (ids %v), было %v", events, want.Checks[i].ID, got.Checks[i].Items, want.Checks[i].Items, ids, prev.Checks[i].Items)
				}
			}
			t.Fatalf("после %d событий частичный итог не равен полному: сводка %v", events, reflect.DeepEqual(got.Summary, want.Summary))
		}
		prev = got
	}
	t.Logf("событий %d: частичных пересчётов %d, полных %d", events, partial, fallback)
	if partial < fallback*3 {
		t.Errorf("частичных пересчётов %d, полных %d: частичный путь почти не работает", partial, fallback)
	}
}

func TestRescoreNeedsFull(t *testing.T) {
	rs := benchSnapshot(t, 200)
	st := risk.Defaults()
	prev := risk.Score(rs, st, rs.AsOf)
	ctx := context.Background()

	if _, err := risk.Rescore(ctx, prev, touchRows(rs, []int64{-5}), []int64{-5}, st, risk.Options{}); err != risk.ErrNeedFull {
		t.Errorf("клиента нет в прежнем результате: ошибка %v, ждали ErrNeedFull", err)
	}
	id := rs.Accounts[0].ID
	sub := touchRows(rs, []int64{id})
	sub.AsOf = sub.AsOf.Add(time.Hour)
	if _, err := risk.Rescore(ctx, prev, sub, []int64{id}, st, risk.Options{}); err != risk.ErrNeedFull {
		t.Errorf("asOf снимка сменился: ошибка %v, ждали ErrNeedFull", err)
	}
	sub = touchRows(rs, []int64{id})
	sub.Accounts = nil
	if _, err := risk.Rescore(ctx, prev, sub, []int64{id}, st, risk.Options{}); err != risk.ErrNeedFull {
		t.Errorf("счёт пропал из строк: ошибка %v, ждали ErrNeedFull", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := risk.Rescore(cancelled, prev, touchRows(rs, []int64{id}), []int64{id}, st, risk.Options{}); err == nil {
		t.Error("отменённый контекст: ошибки нет")
	}
	// повтор клиента и порядок ids не влияют на итог
	ids := []int64{rs.Accounts[3].ID, id, rs.Accounts[3].ID}
	a, err := risk.Rescore(ctx, prev, touchRows(rs, ids), ids, st, risk.Options{})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(ids)
	if b, _ := risk.Rescore(ctx, prev, touchRows(rs, ids), ids, st, risk.Options{}); !reflect.DeepEqual(a, b) {
		t.Error("порядок и повторы ids изменили итог")
	}
	if !reflect.DeepEqual(a, prev) {
		t.Error("данные не менялись, а итог другой")
	}
}

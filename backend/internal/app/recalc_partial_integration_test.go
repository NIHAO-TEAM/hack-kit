//go:build integration

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"testing"
	"time"

	"example.com/app/internal/analytics"
	"example.com/app/internal/apitest"
	"example.com/app/internal/app"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/risk"
	"example.com/app/internal/source"
	"example.com/app/tools/snapgen"
)

// partialRig — стенд со снимком на n клиентов и воркером, у которого частичный пересчёт включён.
type partialRig struct {
	*rig
	rc  *app.Recalc
	rng *rand.Rand
}

func newPartialRig(t *testing.T, accounts int, fullEvery time.Duration) *partialRig {
	t.Helper()
	r := newRig(t, false)
	body, err := snapgen.Generate(snapgen.Options{Accounts: accounts, Seed: 21})
	if err != nil {
		t.Fatal(err)
	}
	c := r.WithBearer(t, apitest.SourceToken)
	if res, err := c.PutSnapshotWithBodyWithResponse(context.Background(), "application/json", bytes.NewReader(body)); err != nil || res.StatusCode() != 200 {
		t.Fatalf("PUT snapshot: %v %v", err, res)
	}
	rc := app.NewRecalc(r.Pool, r.Bus, r.Clock).WithFullEvery(fullEvery)
	ctx, cancel := context.WithCancel(context.Background())
	done := rc.Start(ctx)
	t.Cleanup(func() {
		cancel()
		<-done
	})
	waitLatest(t, rc, "первый пересчёт", func() bool { return rc.Engine.Latest() != nil })
	return &partialRig{rig: r, rc: rc, rng: rand.New(rand.NewSource(5))}
}

// event меняет строки клиентов в базе, заводит версию снимка и объявляет source.changed — как писатель потока.
func (p *partialRig) event(t *testing.T, accounts []int64) int64 {
	t.Helper()
	ctx := context.Background()
	for _, id := range accounts {
		if _, err := p.Pool.Exec(ctx, `UPDATE accounts SET balance = $1 WHERE id = $2`, int64(p.rng.Intn(60_000)), id); err != nil {
			t.Fatal(err)
		}
		status := []string{"active", "suspended", "prolong_wait", "blocked", "deleted"}[p.rng.Intn(5)]
		if _, err := p.Pool.Exec(ctx, `UPDATE services SET status = $1, "autoProlong" = $2 WHERE id = (SELECT id FROM services WHERE "ownerId" = $3 ORDER BY id LIMIT 1)`,
			status, p.rng.Intn(2) == 0, id); err != nil {
			t.Fatal(err)
		}
	}
	var version int64
	if err := p.Pool.QueryRow(ctx, `INSERT INTO snapshots (as_of, loaded_at, source)
		SELECT as_of, now(), 'emulator' FROM snapshots ORDER BY version DESC LIMIT 1 RETURNING version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	slices.Sort(accounts)
	p.Bus.Publish(source.TopicChanged, source.Change{Version: version, Accounts: accounts})
	return version
}

func (p *partialRig) pick(n int) []int64 {
	seen := map[int64]bool{}
	var out []int64
	for len(out) < n {
		id := int64(1 + p.rng.Intn(150))
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// full — что дал бы полный пересчёт по тому, что сейчас в базе.
func (p *partialRig) full(t *testing.T) risk.Result {
	t.Helper()
	snap, _, err := source.NewReader(p.Pool).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	rs := app.ToRiskSnapshot(snap)
	return risk.Score(rs, risk.Defaults(), rs.AsOf)
}

// 2.2/2.3: события по клиентам пересчитывают только их; итог и очередь равны полному пересчёту.
func TestPartialRecalcEqualsFull(t *testing.T) {
	p := newPartialRig(t, 150, time.Hour)
	reads := p.rc.TableReads()

	partial, rounds := 0, 25
	for round := 0; round < 25; round++ {
		ids := p.pick(1 + p.rng.Intn(6))
		version := p.event(t, slices.Clone(ids))
		waitLatest(t, p.rc, "частичный пересчёт", func() bool { return p.rc.Engine.Latest().DataVersion >= version })

		got := p.rc.Engine.Latest()
		if got.Partial {
			partial++
			if got.Customers != len(ids) {
				t.Errorf("раунд %d: пересчитано %d клиентов, событие назвало %d", round, got.Customers, len(ids))
			}
		}
		want := p.full(t)
		if !reflect.DeepEqual(got.Result, want) {
			t.Fatalf("раунд %d: частичный итог не равен полному: сводка %+v, ждали %+v", round, got.Result.Summary, want.Summary)
		}

		// очередь: те же строки в том же порядке, что собрал бы полный пересчёт
		pf := p.rc.Portfolio()
		snap, _, _ := source.NewReader(p.Pool).Load(context.Background())
		fresh := app.BuildPortfolioForTest(want, app.ToRiskSnapshot(snap), pf.Version)
		sorts := []gen.ListCustomersParamsSort{"id", "email", "balance", "totalReplenished", "score", "expectedLoss"}
		for _, key := range sorts {
			if !reflect.DeepEqual(listIDs(t, pf, key, true), listIDs(t, fresh, key, true)) ||
				!reflect.DeepEqual(listIDs(t, pf, key, false), listIDs(t, fresh, key, false)) {
				t.Fatalf("раунд %d: порядок %s после частичного пересчёта не равен порядку с нуля", round, key)
			}
		}
		if !reflect.DeepEqual(pf.Customers, fresh.Customers) {
			t.Fatalf("раунд %d: строки портфеля после частичного пересчёта не равны сборке с нуля", round)
		}
	}
	// В маленьком портфеле список первых услуг проверки качества часто упирается в клиентов события,
	// и воркер честно считает полный; на 100 тыс. это редкость. Главное: частичных большинство.
	if partial < rounds*2/3 {
		t.Errorf("частичных пересчётов %d из %d", partial, rounds)
	}
	if got, want := p.rc.TableReads()-reads, int64(rounds-partial); got != want {
		t.Errorf("таблицы снимка прочитаны целиком %d раз, полных пересчётов %d: частичный пересчёт их не читает", got, want)
	}
}

// 2.4: RECALC_FULL_EVERY = 0 — частичный путь выключен; событие снимка целиком даёт полный пересчёт.
func TestPartialRecalcOffAndFullOnSnapshot(t *testing.T) {
	off := newPartialRig(t, 100, 0)
	version := off.event(t, []int64{3})
	waitLatest(t, off.rc, "пересчёт", func() bool { return off.rc.Engine.Latest().DataVersion >= version })
	if got := off.rc.Engine.Latest(); got.Partial || got.Customers != 100 {
		t.Errorf("при нулевом периоде пересчёт должен быть полным: partial=%v, клиентов %d", got.Partial, got.Customers)
	}

	on := newPartialRig(t, 100, time.Hour)
	version = on.event(t, []int64{3})
	waitLatest(t, on.rc, "частичный", func() bool { return on.rc.Engine.Latest().DataVersion >= version })
	if got := on.rc.Engine.Latest(); !got.Partial || got.Customers != 1 {
		t.Errorf("событие по одному клиенту: partial=%v, клиентов %d, ждали 1", got.Partial, got.Customers)
	}
	var v int64
	if err := on.Pool.QueryRow(context.Background(), `INSERT INTO snapshots (as_of, loaded_at, source)
		SELECT as_of, now(), 'api' FROM snapshots ORDER BY version DESC LIMIT 1 RETURNING version`).Scan(&v); err != nil {
		t.Fatal(err)
	}
	on.Bus.Publish(source.TopicChanged, source.Change{Version: v})
	waitLatest(t, on.rc, "полный после снимка", func() bool { return on.rc.Engine.Latest().DataVersion >= v })
	if got := on.rc.Engine.Latest(); got.Partial || got.Customers != 100 {
		t.Errorf("снимок целиком: partial=%v, клиентов %d, ждали полный на 100", got.Partial, got.Customers)
	}
}

// 2.4: раз в RECALC_FULL_EVERY пересчёт полный, даже если события частичные.
func TestPartialRecalcFullEveryPeriod(t *testing.T) {
	p := newPartialRig(t, 100, time.Minute)
	p.Clock.Advance(2 * time.Minute) // воркер считает время по часам стенда
	version := p.event(t, []int64{5})
	waitLatest(t, p.rc, "пересчёт", func() bool { return p.rc.Engine.Latest().DataVersion >= version })
	if got := p.rc.Engine.Latest(); got.Partial {
		t.Error("прошло больше периода с полного пересчёта, а пересчёт частичный")
	}
}

type feed struct{ pf *analytics.Portfolio }

func (f feed) Portfolio() *analytics.Portfolio { return f.pf }

// listIDs — порядок клиентов в очереди по сортировке, как его отдаёт API (до 200 строк за запрос).
func listIDs(t *testing.T, pf *analytics.Portfolio, key gen.ListCustomersParamsSort, desc bool) []int64 {
	t.Helper()
	order := gen.Asc
	if desc {
		order = gen.Desc
	}
	limit := 200
	rec := httptest.NewRecorder()
	analytics.New(nil, nil, clock.NewFake(time.Now())).WithFeed(feed{pf}).
		ListCustomers(rec, httptest.NewRequest(http.MethodGet, "/api/v1/customers", nil),
			gen.ListCustomersParams{Sort: &key, Order: &order, Limit: &limit})
	var out struct {
		Items []gen.CustomerRow `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("список: %d %v", rec.Code, err)
	}
	ids := make([]int64, len(out.Items))
	for i, r := range out.Items {
		ids[i] = r.Id
	}
	return ids
}

// Замер для docs/load: задержка «событие по одному клиенту → готов новый результат» на 100 тыс. клиентов,
// с частичным пересчётом и без него. Долгий, поэтому только по LATENCY_100K=1:
// LATENCY_100K=1 go test -tags integration ./internal/app -run TestLatencyOneEvent100k -v
func TestLatencyOneEvent100k(t *testing.T) {
	if os.Getenv("LATENCY_100K") == "" {
		t.Skip("замер включается LATENCY_100K=1")
	}
	for _, mode := range []struct {
		name string
		full time.Duration
	}{{"полный (RECALC_FULL_EVERY=0)", 0}, {"частичный", time.Hour}} {
		p := newPartialRig(t, 100_000, mode.full)
		var took []time.Duration
		for i := 0; i < 15; i++ {
			start := time.Now() // от правки строк в базе до готового результата
			version := p.event(t, []int64{int64(1 + p.rng.Intn(100_000))})
			waitLatest(t, p.rc, "пересчёт", func() bool { return p.rc.Engine.Latest().DataVersion >= version })
			took = append(took, time.Since(start))
		}
		slices.Sort(took)
		t.Logf("%s: медиана %v, макс %v (15 событий)", mode.name, took[len(took)/2], took[len(took)-1])
	}
}

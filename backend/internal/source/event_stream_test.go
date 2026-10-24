package source_test

import (
	"context"
	"math/rand"
	"net/http"
	"sort"
	"testing"
	"time"

	"example.com/app/internal/apitest"
	"example.com/app/internal/app"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/risk"
	"example.com/app/internal/source"
)

const eventStreamEvents = 2500

// TestEventStreamConsistency подаёт поток событий пользователей через обработчики правок
// и проверяет свойство: итоговые уровни, ожидаемая потеря и очередь равны пересчёту с нуля
// над итоговым снимком. Скорость только печатается (docs/load/2026-10-24-events-2500.md).
func TestEventStreamConsistency(t *testing.T) {
	s := apitest.New(t)
	rc := s.StartRecalc(t)
	analyst := s.As(t, "analyst")
	marketer := s.As(t, "marketer")
	ctx := context.Background()
	put(t, analyst, organizers(t))

	snap, _, err := source.NewReader(s.Pool).Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	status := map[int64]gen.ServiceStatus{}
	var live []int64
	for _, v := range snap.Services {
		status[v.ID] = gen.ServiceStatus(v.Status)
		if v.Status == "active" || v.Status == "suspended" || v.Status == "prolong_wait" {
			live = append(live, v.ID)
		}
	}
	accounts := make([]int64, len(snap.Accounts))
	for i, a := range snap.Accounts {
		accounts[i] = a.ID
	}
	if len(live) == 0 || len(accounts) == 0 {
		t.Fatal("в снимке нет живых услуг или счетов")
	}

	rng := rand.New(rand.NewSource(20261024))
	lat := make([]time.Duration, 0, eventStreamEvents)
	timed := func(call func() (int, error)) {
		t.Helper()
		t0 := time.Now()
		code, err := call()
		lat = append(lat, time.Since(t0))
		if err != nil || code != http.StatusOK {
			t.Fatalf("событие %d: статус %d, ошибка %v", len(lat), code, err)
		}
	}
	patchService := func(id int64, p gen.ServicePatch) {
		timed(func() (int, error) {
			r, err := analyst.PatchServiceWithResponse(ctx, id, p)
			if err != nil {
				return 0, err
			}
			return r.StatusCode(), nil
		})
	}

	start := time.Now()
	for i := 0; i < eventStreamEvents; i++ {
		id := live[rng.Intn(len(live))]
		switch rng.Intn(6) {
		case 0, 1: // пополнение баланса
			bal := int64(rng.Intn(500_00))
			acc := accounts[rng.Intn(len(accounts))]
			timed(func() (int, error) {
				r, err := analyst.PatchAccountWithResponse(ctx, acc, gen.AccountPatch{Balance: &bal})
				if err != nil {
					return 0, err
				}
				return r.StatusCode(), nil
			})
		case 2: // включение автопродления
			on := true
			patchService(id, gen.ServicePatch{AutoProlong: &on})
		case 3: // выключение автопродления
			off := false
			patchService(id, gen.ServicePatch{AutoProlong: &off})
		case 4: // остановка и возобновление услуги
			next := gen.Suspended
			if status[id] == gen.Suspended {
				next = gen.Active
			}
			status[id] = next
			patchService(id, gen.ServicePatch{Status: &next})
		default: // неудачное продление: услуга ждёт оплаты, потом возвращается в работу
			next := gen.ProlongWait
			if status[id] == gen.ProlongWait {
				next = gen.Active
			}
			status[id] = next
			patchService(id, gen.ServicePatch{Status: &next})
		}
	}
	took := time.Since(start)

	// Ждём пересчёт по версии снимка, а не по времени.
	want := func() int64 {
		st, err := analyst.GetSourceStatusWithResponse(ctx)
		if err != nil || st.JSON200 == nil {
			t.Fatalf("статус источника: %v", err)
		}
		return st.JSON200.Version
	}()
	deadline := time.Now().Add(30 * time.Second)
	for {
		if c := rc.Engine.Latest(); c != nil && c.DataVersion == want {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("пересчёт по версии %d не завершился за 30 с", want)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Независимый пересчёт с нуля: итоговый снимок из БД, умолчания модели, те же функции ядра.
	final, ver, err := source.NewReader(s.Pool).Load(ctx)
	if err != nil || ver != want {
		t.Fatalf("итоговый снимок: версия %d (ждали %d), %v", ver, want, err)
	}
	rs := app.ToRiskSnapshot(final)
	ref := risk.Score(rs, risk.Defaults(), rs.AsOf)

	ov := rc.Overview()
	for lvl, n := range ref.Summary.Levels {
		if ov.Levels[lvl] != n {
			t.Errorf("уровень %s: поток %d, пересчёт с нуля %d", lvl, ov.Levels[lvl], n)
		}
	}
	if ov.ExpectedLossCents != ref.Summary.ExpectedLossCents {
		t.Errorf("ожидаемая потеря: поток %d, пересчёт с нуля %d", ov.ExpectedLossCents, ref.Summary.ExpectedLossCents)
	}

	// Очередь: каждая строка равна пересчёту с нуля, порядок — по убыванию ожидаемой потери.
	refByID := map[int64]risk.Customer{}
	for _, c := range ref.Customers {
		refByID[c.ID] = c
	}
	limit := 200
	sortBy, order := gen.ExpectedLoss, gen.Desc
	page, err := marketer.ListCustomersWithResponse(ctx, &gen.ListCustomersParams{Sort: &sortBy, Order: &order, Limit: &limit})
	if err != nil || page.JSON200 == nil {
		t.Fatalf("очередь: %v %d", err, page.StatusCode())
	}
	if page.JSON200.Total != len(ref.Customers) {
		t.Errorf("клиентов в очереди %d, в пересчёте с нуля %d", page.JSON200.Total, len(ref.Customers))
	}
	prev := int64(-1)
	for i, row := range page.JSON200.Items {
		c, ok := refByID[row.Id]
		if !ok {
			t.Errorf("строка %d: клиента %d нет в пересчёте с нуля", i, row.Id)
			continue
		}
		if row.ExpectedLossCents != c.ExpectedLossCents || (c.Level != nil && string(row.Level) != *c.Level) {
			t.Errorf("клиент %d: очередь %s/%d, пересчёт %v/%d", row.Id, row.Level, row.ExpectedLossCents, c.Level, c.ExpectedLossCents)
		}
		if prev >= 0 && row.ExpectedLossCents > prev {
			t.Errorf("порядок очереди нарушен на строке %d", i)
		}
		prev = row.ExpectedLossCents
	}

	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	p95 := lat[len(lat)*95/100]
	t.Logf("событий %d за %v (%.0f событий/с), p95 ответа %v, версий снимка %d", eventStreamEvents,
		took.Round(time.Millisecond), float64(eventStreamEvents)/took.Seconds(), p95.Round(100*time.Microsecond), want)
}

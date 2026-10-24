//go:build integration

package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/apitest"
	"example.com/app/internal/app"
	"example.com/app/internal/platform/db"
	"example.com/app/internal/risk"
	"example.com/app/internal/source"
)

type rig struct {
	*apitest.Stand
	dsn string
}

func newRig(t *testing.T, seed bool) *rig {
	t.Helper()
	st := apitest.New(t)
	if err := source.OnStart(context.Background(), st.Pool, st.Bus, st.Clock, seed); err != nil {
		t.Fatal(err)
	}
	return &rig{Stand: st, dsn: st.Pool.Config().ConnString()}
}

// reload грузит сид организаторов через PUT /source/snapshot: каждая загрузка — новая версия снимка.
func (r *rig) reload(t *testing.T) {
	t.Helper()
	body, err := os.ReadFile("../source/seed/organizers.json")
	if err != nil {
		t.Fatal(err)
	}
	c := r.WithBearer(t, apitest.SourceToken)
	res, err := c.PutSnapshotWithBodyWithResponse(context.Background(), "application/json", bytes.NewReader(body))
	if err != nil || res.StatusCode() != 200 {
		t.Errorf("PUT snapshot: %v %d %s", err, res.StatusCode(), res.Body)
	}
}

// start запускает воркер на пуле rig и останавливает его при завершении теста.
func (r *rig) start(t *testing.T, pool *pgxpool.Pool) *app.Recalc {
	t.Helper()
	rc := app.NewRecalc(pool, r.Bus, r.Clock)
	ctx, cancel := context.WithCancel(context.Background())
	done := rc.Start(ctx)
	t.Cleanup(func() {
		cancel()
		<-done
	})
	return rc
}

func waitLatest(t *testing.T, rc *app.Recalc, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("не дождались: %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func saveSettings(t *testing.T, rc *app.Recalc, st risk.Settings) int64 {
	t.Helper()
	body, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	v, err := rc.Settings.Save(context.Background(), body, "analyst")
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// Уровни 3 / 2 / 1 / 4 из демо-сценария дают все признаки модели; пока в реестре pwait и noauto,
// воркер обязан отдать ровно то, что считает ядро на этом снимке, и 4 потерянных клиента.
func TestStartOnSeedMatchesScore(t *testing.T) {
	r := newRig(t, true)
	rc := r.start(t, r.Pool)
	waitLatest(t, rc, "пересчёт при старте", func() bool { return rc.Engine.Latest() != nil })

	got := rc.Engine.Latest()
	snap, _, err := source.NewReader(r.Pool).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	rs := app.ToRiskSnapshot(snap)
	want := risk.Score(rs, risk.Defaults(), rs.AsOf)
	if !reflect.DeepEqual(got.Result, want) {
		t.Errorf("результат воркера расходится с risk.Score")
	}
	if n := len(got.Result.Customers); n != 10 {
		t.Errorf("клиентов %d, ждали 10", n)
	}
	if lost := got.Result.Summary.Levels[risk.StateLost]; lost != 4 {
		t.Errorf("потеряно %d, ждали 4", lost)
	}
	if got.DataVersion != 1 || got.SettingsVersion != 0 || got.Version != 1 {
		t.Errorf("версии: %+v", got)
	}
	if wantAsOf := time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC); !got.Result.AsOf.Equal(wantAsOf) {
		t.Errorf("asOf = %v", got.Result.AsOf)
	}
}

func TestSettingsSurviveRestart(t *testing.T) {
	r := newRig(t, true)
	rc := r.start(t, r.Pool)
	st := risk.Defaults()
	st.High = 90
	if v := saveSettings(t, rc, st); v != 1 {
		t.Fatalf("версия %d", v)
	}

	// «Перезапуск»: новый пул и новый воркер на той же базе.
	pool2, err := db.Open(context.Background(), r.dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool2.Close)
	rc2 := r.start(t, pool2)
	waitLatest(t, rc2, "пересчёт после перезапуска", func() bool { return rc2.Engine.Latest() != nil })
	if got := rc2.Engine.Latest(); got.SettingsVersion != 1 {
		t.Errorf("версия настроек после перезапуска = %d, ждали 1", got.SettingsVersion)
	}
	body, v, err := rc2.Settings.Current(context.Background())
	var back risk.Settings
	if err != nil || v != 1 || json.Unmarshal(body, &back) != nil || back.High != 90 {
		t.Errorf("Current: %s v=%d err=%v", body, v, err)
	}
}

// TestRaceSettingsAndSnapshot: настройки и снимок меняются параллельно; итог соответствует
// последней паре версий (запускать с -race).
func TestRaceSettingsAndSnapshot(t *testing.T) {
	r := newRig(t, true)
	rc := r.start(t, r.Pool)
	waitLatest(t, rc, "пересчёт при старте", func() bool { return rc.Engine.Latest() != nil })

	var wg sync.WaitGroup
	var last int64
	var mu sync.Mutex
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 8; i++ {
			st := risk.Defaults()
			st.High = float64(75 + i)
			v := saveSettings(t, rc, st)
			mu.Lock()
			if v > last {
				last = v
			}
			mu.Unlock()
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 4; i++ {
			// Перезагрузка того же сида: каждая даёт новую версию снимка.
			r.reload(t)
		}
	}()
	wg.Wait()

	var dataV int64
	if err := r.Pool.QueryRow(context.Background(), "SELECT max(version) FROM snapshots").Scan(&dataV); err != nil {
		t.Fatal(err)
	}
	waitLatest(t, rc, "последняя пара версий", func() bool {
		l := rc.Engine.Latest()
		return l.DataVersion == dataV && l.SettingsVersion == last
	})
	l := rc.Engine.Latest()
	if lv := l.Result.Summary.Levels; lv[risk.LevelHigh]+lv[risk.LevelMedium]+lv[risk.LevelLow] == 0 {
		t.Errorf("пустой результат: %v", lv)
	}
}

// Смена настроек не перечитывает таблицы: версия снимка та же, берётся собранный снимок.
// Новая версия данных (загрузка снимка) читает таблицы заново, и очередь показывает новые данные.
func TestRecalcSkipsTableReadWhenSnapshotVersionSame(t *testing.T) {
	r := newRig(t, true)
	rc := r.start(t, r.Pool)
	waitLatest(t, rc, "первый пересчёт", func() bool { return rc.Engine.Latest() != nil })
	if n := rc.TableReads(); n != 1 {
		t.Fatalf("первый пересчёт: чтений таблиц = %d, ждали 1", n)
	}
	first := rc.Engine.Latest().Version

	st := risk.Defaults()
	for id := range st.Weights {
		st.Weights[id] = 0.5
		break
	}
	saveSettings(t, rc, st)
	waitLatest(t, rc, "пересчёт по настройкам", func() bool { return rc.Engine.Latest().Version > first })
	if n := rc.TableReads(); n != 1 {
		t.Errorf("после смены настроек чтений таблиц = %d, ждали 1 (без перечитывания)", n)
	}

	before := rc.Engine.Latest().Version
	r.reload(t)
	waitLatest(t, rc, "пересчёт по новой версии", func() bool { return rc.Engine.Latest().Version > before })
	if n := rc.TableReads(); n != 2 {
		t.Errorf("после новой версии снимка чтений таблиц = %d, ждали 2", n)
	}
}

package model

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/httpx"
)

// SensitivityTimeout — потолок ожидания расчёта чувствительности; дольше — ответ 503.
const SensitivityTimeout = 10 * time.Second

// ErrNoData — нет снимка данных, считать не на чем.
var ErrNoData = errors.New("model: нет снимка данных")

// ShiftResult — итог одного сдвига веса.
type ShiftResult struct {
	LevelChanges    int
	TopThreeChanged bool
	LossDeltaCents  int64
}

// FeatureSensitivity — чувствительность к весу одного признака.
type FeatureSensitivity struct {
	ID       string
	Weight   float64
	Down, Up ShiftResult
}

// Sensitivity — чувствительность очереди к весам на снимке SnapshotVersion при настройках SettingsVersion.
type Sensitivity struct {
	Features        []FeatureSensitivity
	Sampled         bool
	Customers       int
	SnapshotVersion int64
	SettingsVersion int64
}

type sensKey struct{ snapshot, settings int64 }

type sensCall struct {
	done chan struct{}
	res  Sensitivity
	err  error
}

// sensitivityCache держит последний результат по версиям снимка и настроек. Расчёт один за раз:
// повторный запрос на тех же версиях ждёт тот же расчёт, а не запускает свой.
type sensitivityCache struct {
	mu     sync.Mutex
	key    sensKey
	res    *Sensitivity
	flight map[sensKey]*sensCall
}

// get возвращает результат для ключа: из кэша, из идущего расчёта или запустив compute.
// compute не прерывается по таймауту — он дойдёт до конца и заполнит кэш для следующего запроса.
func (c *sensitivityCache) get(ctx context.Context, key sensKey, compute func() (Sensitivity, error)) (Sensitivity, error) {
	c.mu.Lock()
	if c.res != nil && c.key == key {
		r := *c.res
		c.mu.Unlock()
		return r, nil
	}
	call, ok := c.flight[key]
	if !ok {
		if c.flight == nil {
			c.flight = map[sensKey]*sensCall{}
		}
		call = &sensCall{done: make(chan struct{})}
		c.flight[key] = call
		go func() {
			call.res, call.err = compute()
			c.mu.Lock()
			delete(c.flight, key)
			if call.err == nil {
				res := call.res
				c.key, c.res = sensKey{res.SnapshotVersion, res.SettingsVersion}, &res
			}
			c.mu.Unlock()
			close(call.done)
		}()
	}
	c.mu.Unlock()
	select {
	case <-call.done:
		return call.res, call.err
	case <-ctx.Done():
		return Sensitivity{}, ctx.Err()
	}
}

// GetModelSensitivity — GET /api/v1/model/sensitivity. Только чтение: настройки не пишутся.
func (a *ModelAPI) GetModelSensitivity(w http.ResponseWriter, r *http.Request) {
	if a.store == nil || a.ports.Sensitivity == nil {
		httpx.NotImplemented(w, r)
		return
	}
	s, rec, err := a.current(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	body, err := s.Marshal()
	if err != nil {
		fail(w, r, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), SensitivityTimeout)
	defer cancel()
	snap, err := a.ports.SnapshotVersion(ctx)
	if err == nil {
		var res Sensitivity
		res, err = a.sens.get(ctx, sensKey{snap, rec.Version}, func() (Sensitivity, error) {
			// расчёт живёт дольше запроса: его контекст не привязан к запросу
			out, err := a.ports.Sensitivity(context.Background(), body)
			out.SettingsVersion = rec.Version
			return out, err
		})
		if err == nil {
			writeJSON(w, toAPISensitivity(res))
			return
		}
	}
	switch {
	case errors.Is(err, ErrNoData):
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "нет снимка данных: считать чувствительность не на чем")
	case errors.Is(err, context.DeadlineExceeded):
		httpx.WriteProblem(w, r, http.StatusServiceUnavailable, "чувствительность считается дольше 10 секунд, повторите запрос позже")
	case errors.Is(err, context.Canceled):
		// клиент ушёл, отвечать некому
	default:
		fail(w, r, err)
	}
}

func toAPISensitivity(s Sensitivity) gen.ModelSensitivity {
	shift := func(x ShiftResult) gen.WeightShift {
		return gen.WeightShift{LevelChanges: x.LevelChanges, TopThreeChanged: x.TopThreeChanged, LossDeltaCents: x.LossDeltaCents}
	}
	out := gen.ModelSensitivity{
		Features:        make([]gen.FeatureSensitivity, len(s.Features)),
		ShiftPercent:    20,
		Sampled:         s.Sampled,
		Customers:       s.Customers,
		SnapshotVersion: s.SnapshotVersion,
		SettingsVersion: s.SettingsVersion,
	}
	for i, f := range s.Features {
		out.Features[i] = gen.FeatureSensitivity{Id: f.ID, Weight: f.Weight, Down: shift(f.Down), Up: shift(f.Up)}
	}
	return out
}

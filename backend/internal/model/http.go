// Package model — настройки и коэффициенты модели: чтение, правка с пересчётом, предпросмотр.
package model

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"example.com/app/internal/audit"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/httpx"
)

// ModelAPI обслуживает пути /model/*.
type ModelAPI struct {
	store *Store
	ports Ports
	clk   clock.Clock
	audit audit.Recorder
	sens  sensitivityCache
}

// WithAudit подключает журнал действий: каждая сохранённая правка настроек пишется с «было → стало».
func (a *ModelAPI) WithAudit(r audit.Recorder) *ModelAPI {
	a.audit = r
	return a
}

// New создаёт обработчик модуля. store nil (тесты без базы) — операции отвечают 501.
func New(store *Store, ports Ports, clk clock.Clock) *ModelAPI {
	return &ModelAPI{store: store, ports: ports, clk: clk}
}

// current читает действующие настройки; ничего не сохраняли — умолчания, версия 0.
func (a *ModelAPI) current(r *http.Request) (Settings, Record, error) {
	rec, err := a.store.Latest(r.Context())
	if err != nil {
		return Settings{}, rec, err
	}
	body := rec.Body
	if body == nil {
		body = a.ports.Defaults()
	}
	s, err := Parse(body)
	if err != nil {
		return Settings{}, rec, err
	}
	if s.AsOfMode == "" {
		s.AsOfMode = AsOfSnapshot
	}
	return s, rec, nil
}

func toAPI(s Settings, rec Record) gen.ModelSettings {
	out := gen.ModelSettings{
		Weights:     s.Weights,
		HorizonDays: s.HorizonDays,
		PriceBasis:  gen.ModelSettingsPriceBasis(s.PriceBasis),
		AsOfMode:    gen.ModelSettingsAsOfMode(s.AsOfMode),
		Version:     rec.Version,
	}
	out.Thresholds.High = s.High
	out.Thresholds.Medium = s.Medium
	if rec.Version > 0 {
		by, at := rec.UpdatedBy, rec.UpdatedAt
		out.UpdatedBy, out.UpdatedAt = &by, &at
	}
	return out
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("model: запись ответа: %v", err)
	}
}

func fail(w http.ResponseWriter, r *http.Request, err error) {
	httpx.WriteFailure(w, r, err, "ошибка работы с настройками модели")
}

// GetModelSettings — GET /api/v1/model/settings.
func (a *ModelAPI) GetModelSettings(w http.ResponseWriter, r *http.Request) {
	if a.store == nil {
		httpx.NotImplemented(w, r)
		return
	}
	s, rec, err := a.current(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, toAPI(s, rec))
}

func toPatch(in gen.ModelSettingsPatch) Patch {
	p := Patch{HorizonDays: in.HorizonDays}
	if in.Weights != nil {
		p.Weights = *in.Weights
	}
	if in.Thresholds != nil {
		p.High, p.Medium = in.Thresholds.High, in.Thresholds.Medium
	}
	if in.PriceBasis != nil {
		v := string(*in.PriceBasis)
		p.PriceBasis = &v
	}
	if in.AsOfMode != nil {
		v := string(*in.AsOfMode)
		p.AsOfMode = &v
	}
	return p
}

// PatchModelSettings — PATCH /api/v1/model/settings. Частичная правка сливается поверх
// действующих настроек. dryRun=true ничего не сохраняет и отвечает последствиями.
func (a *ModelAPI) PatchModelSettings(w http.ResponseWriter, r *http.Request, params gen.PatchModelSettingsParams) {
	if a.store == nil {
		httpx.NotImplemented(w, r)
		return
	}
	var in gen.ModelSettingsPatch
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "тело запроса не разобрано: "+err.Error())
		return
	}
	cur, _, err := a.current(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	next := cur.Apply(toPatch(in))
	if ps := a.ports.Problems(next); len(ps) > 0 {
		errs := make([]httpx.FieldError, len(ps))
		for i, p := range ps {
			errs[i] = httpx.FieldError{Field: p.Field, Message: p.Message}
		}
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "настройки не прошли проверку", errs...)
		return
	}
	nextBody, err := next.Marshal()
	if err != nil {
		fail(w, r, err)
		return
	}

	if params.DryRun != nil && *params.DryRun {
		curBody, err := cur.Marshal()
		if err != nil {
			fail(w, r, err)
			return
		}
		d, err := a.ports.Preview(r.Context(), curBody, nextBody)
		if err != nil {
			fail(w, r, err)
			return
		}
		if d.LevelChanges == nil {
			d.LevelChanges = map[string]int{}
		}
		customers := make([]gen.DryRunCustomer, len(d.Customers))
		for i, c := range d.Customers {
			customers[i] = gen.DryRunCustomer{
				Id: c.ID, Name: c.Name, LevelFrom: c.LevelFrom, LevelTo: c.LevelTo,
				ScoreFrom: c.ScoreFrom, ScoreTo: c.ScoreTo, ExpectedLossDeltaCents: c.ExpectedLossDeltaCents,
			}
		}
		writeJSON(w, gen.DryRunResult{
			LevelChanges:           d.LevelChanges,
			ExpectedLossDeltaCents: d.ExpectedLossDeltaCents,
			Customers:              &customers,
			CustomersTotal:         &d.CustomersTotal,
		})
		return
	}

	by := a.ports.Actor(r.Context())
	version, err := a.store.Save(r.Context(), nextBody, by)
	if err != nil {
		fail(w, r, err)
		return
	}
	if a.audit != nil {
		if changes := Diff(cur, next); len(changes) > 0 {
			a.audit.Record(r.Context(), audit.Entry{
				Actor: by, Action: audit.ActionSettingsChange, Summary: "Изменены настройки модели", Changes: changes,
			})
		}
	}
	saved, err := a.store.Latest(r.Context())
	if err != nil || saved.Version < version {
		saved = Record{Version: version, UpdatedBy: by, UpdatedAt: a.clk.Now()}
	}
	writeJSON(w, toAPI(next, saved))
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

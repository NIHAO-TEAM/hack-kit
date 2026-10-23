// Package model — настройки и коэффициенты модели: чтение, правка с пересчётом, предпросмотр.
package model

import (
	"encoding/json"
	"log"
	"net/http"

	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/httpx"
)

// ModelAPI обслуживает пути /model/*.
type ModelAPI struct {
	store *Store
	ports Ports
	clk   clock.Clock
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
	log.Printf("model: %v", err)
	httpx.WriteProblem(w, r, http.StatusInternalServerError, "ошибка работы с настройками модели")
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
	saved, err := a.store.Latest(r.Context())
	if err != nil || saved.Version < version {
		saved = Record{Version: version, UpdatedBy: by, UpdatedAt: a.clk.Now()}
	}
	writeJSON(w, toAPI(next, saved))
}

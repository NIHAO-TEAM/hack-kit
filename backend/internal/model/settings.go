package model

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Значения asOfMode: считать на момент снимка или на текущее время сервера.
const (
	AsOfSnapshot = "snapshot"
	AsOfServer   = "server"
)

// FieldProblem — ошибка одного поля тела API.
type FieldProblem struct {
	Field   string
	Message string
}

// Settings — тело настроек в хранилище. Форма совпадает с risk.Settings (теги JSON); asOfMode
// ядро не читает, он лежит рядом и ничего не пересчитывает.
type Settings struct {
	Weights     map[string]float64 `json:"weights"`
	High        float64            `json:"high"`
	Medium      float64            `json:"medium"`
	HorizonDays int                `json:"horizonDays"`
	PriceBasis  string             `json:"priceBasis"`
	AsOfMode    string             `json:"asOfMode"`
}

// Patch — частичная правка; nil-поле не меняется, веса сливаются по признакам.
type Patch struct {
	Weights     map[string]float64
	High        *float64
	Medium      *float64
	HorizonDays *int
	PriceBasis  *string
	AsOfMode    *string
}

// Parse разбирает сохранённое тело; пустое — настройки не заданы.
func Parse(body json.RawMessage) (Settings, error) {
	var s Settings
	if len(body) == 0 {
		return s, nil
	}
	if err := json.Unmarshal(body, &s); err != nil {
		return Settings{}, fmt.Errorf("model: разбор настроек: %w", err)
	}
	return s, nil
}

// Marshal сериализует настройки для хранилища.
func (s Settings) Marshal() (json.RawMessage, error) {
	b, err := json.Marshal(s)
	return b, err
}

// Apply возвращает копию настроек с применённой правкой.
func (s Settings) Apply(p Patch) Settings {
	out := s
	out.Weights = make(map[string]float64, len(s.Weights)+len(p.Weights))
	for k, v := range s.Weights {
		out.Weights[k] = v
	}
	for k, v := range p.Weights {
		out.Weights[k] = v
	}
	if p.High != nil {
		out.High = *p.High
	}
	if p.Medium != nil {
		out.Medium = *p.Medium
	}
	if p.HorizonDays != nil {
		out.HorizonDays = *p.HorizonDays
	}
	if p.PriceBasis != nil {
		out.PriceBasis = *p.PriceBasis
	}
	if p.AsOfMode != nil {
		out.AsOfMode = *p.AsOfMode
	}
	return out
}

// Problems проверяет то, что знает сам модуль: режим даты и неизвестные признаки (known — id
// признаков модели). Веса, пороги, горизонт и priceBasis проверяет ядро через Ports.Check.
func (s Settings) Problems(known []string) []FieldProblem {
	var ps []FieldProblem
	if s.AsOfMode != AsOfSnapshot && s.AsOfMode != AsOfServer {
		ps = append(ps, FieldProblem{"asOfMode", fmt.Sprintf("asOfMode %q: нужно %s или %s", s.AsOfMode, AsOfSnapshot, AsOfServer)})
	}
	set := make(map[string]bool, len(known))
	for _, id := range known {
		set[id] = true
	}
	ids := make([]string, 0, len(s.Weights))
	for id := range s.Weights {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !set[id] {
			ps = append(ps, FieldProblem{"weights." + id, fmt.Sprintf("неизвестный признак %q, есть: %s", id, strings.Join(known, ", "))})
		}
	}
	return ps
}

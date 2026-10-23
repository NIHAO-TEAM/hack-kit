package risk

import (
	"errors"
	"fmt"
	"sort"
)

// Значения priceBasis.
const (
	PricePerPeriod = "per_period"
	PricePerMonth  = "per_month"
)

const (
	// fundsHours — окно запаса средств (правило 1).
	fundsHours = 168
	// monthHours — часов в «месяце» для priceBasis = per_month.
	monthHours = 720
)

// Settings — настройки модели.
type Settings struct {
	Weights     map[string]float64 `json:"weights"`
	High        float64            `json:"high"`
	Medium      float64            `json:"medium"`
	HorizonDays int                `json:"horizonDays"`
	PriceBasis  string             `json:"priceBasis"`
}

// Defaults — настройки по умолчанию; веса собираются из реестра признаков.
func Defaults() Settings { return defaultsFrom(features) }

func defaultsFrom(r *featureRegistry) Settings {
	w := map[string]float64{}
	for _, f := range r.all() {
		w[f.ID()] = f.DefaultWeight()
	}
	return Settings{Weights: w, High: 70, Medium: 50, HorizonDays: 30, PriceBasis: PricePerPeriod}
}

// FieldProblem — нарушение в одном поле настроек; Field — путь в теле API («weights.noauto»).
type FieldProblem struct {
	Field   string
	Message string
}

// Problems перечисляет нарушения настроек по полям; пусто — настройки верны.
func (s Settings) Problems() []FieldProblem {
	var ps []FieldProblem
	ids := make([]string, 0, len(s.Weights))
	for id := range s.Weights {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if w := s.Weights[id]; w < 0 || w > 1 {
			ps = append(ps, FieldProblem{"weights." + id, fmt.Sprintf("вес %q = %v вне [0, 1]", id, w)})
		}
	}
	if s.High <= s.Medium {
		ps = append(ps, FieldProblem{"thresholds.high", fmt.Sprintf("порог high (%v) должен быть больше medium (%v)", s.High, s.Medium)})
	}
	if s.HorizonDays < 1 || s.HorizonDays > 365 {
		ps = append(ps, FieldProblem{"horizonDays", fmt.Sprintf("горизонт %d дн. вне 1..365", s.HorizonDays)})
	}
	if s.PriceBasis != PricePerPeriod && s.PriceBasis != PricePerMonth {
		ps = append(ps, FieldProblem{"priceBasis", fmt.Sprintf("priceBasis %q: нужно %s или %s", s.PriceBasis, PricePerPeriod, PricePerMonth)})
	}
	return ps
}

// Validate проверяет настройки: веса в [0, 1], High > Medium, горизонт 1..365, известный priceBasis.
func (s Settings) Validate() error {
	var errs []error
	for _, p := range s.Problems() {
		errs = append(errs, errors.New(p.Message))
	}
	return errors.Join(errs...)
}

// weight — вес признака; нет в настройках — вес по умолчанию.
func (s Settings) weight(f Feature) float64 {
	if w, ok := s.Weights[f.ID()]; ok {
		return w
	}
	return f.DefaultWeight()
}

func (s Settings) horizonHours() int { return s.HorizonDays * 24 }

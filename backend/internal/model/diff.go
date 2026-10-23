package model

import (
	"sort"
	"strconv"

	"example.com/app/internal/audit"
)

func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// Diff перечисляет поля, которые отличаются в настройках до и после правки: было → стало.
// Порядок стабилен: сначала веса по имени признака, затем остальные поля.
func Diff(from, to Settings) []audit.Change {
	var out []audit.Change
	ids := map[string]bool{}
	for id := range from.Weights {
		ids[id] = true
	}
	for id := range to.Weights {
		ids[id] = true
	}
	names := make([]string, 0, len(ids))
	for id := range ids {
		names = append(names, id)
	}
	sort.Strings(names)
	for _, id := range names {
		if from.Weights[id] != to.Weights[id] {
			out = append(out, audit.Change{Field: "weights." + id, From: num(from.Weights[id]), To: num(to.Weights[id])})
		}
	}
	if from.High != to.High {
		out = append(out, audit.Change{Field: "thresholds.high", From: num(from.High), To: num(to.High)})
	}
	if from.Medium != to.Medium {
		out = append(out, audit.Change{Field: "thresholds.medium", From: num(from.Medium), To: num(to.Medium)})
	}
	if from.HorizonDays != to.HorizonDays {
		out = append(out, audit.Change{Field: "horizonDays", From: strconv.Itoa(from.HorizonDays), To: strconv.Itoa(to.HorizonDays)})
	}
	if from.PriceBasis != to.PriceBasis {
		out = append(out, audit.Change{Field: "priceBasis", From: from.PriceBasis, To: to.PriceBasis})
	}
	if from.AsOfMode != to.AsOfMode {
		out = append(out, audit.Change{Field: "asOfMode", From: from.AsOfMode, To: to.AsOfMode})
	}
	return out
}

package risk

import (
	"cmp"
	"math"
	"slices"
)

// FeatureLoss — часть ожидаемой потери, отнесённая к одному признаку (причине).
// Customers — сколько клиентов имеют ненулевую долю по этому признаку.
type FeatureLoss struct {
	ID        string
	LossCents int64
	Customers int
}

// AllocateLoss раскладывает потерю услуги по признакам пропорционально вкладу w·f. Это приближение:
// шумовое ИЛИ не раскладывается на независимые части, поэтому доли не доказывают причинность.
// Каждая доля округляется вниз, остаток до loss получает признак с наибольшим вкладом (при равенстве — с меньшим ID).
// Результат упорядочен как hits; сумма равна loss. Нет вкладов — nil.
func AllocateLoss(loss int64, hits []FeatureHit) []int64 {
	if loss <= 0 || len(hits) == 0 {
		return nil
	}
	// сумма в порядке ID: тот же порядок у оракула, поэтому до последнего бита одно и то же значение
	var buf [16]FeatureHit
	byID := append(buf[:0], hits...)
	slices.SortFunc(byID, func(a, b FeatureHit) int { return cmp.Compare(a.ID, b.ID) })
	var total float64
	for _, h := range byID {
		total += h.Contribution
	}
	if total <= 0 {
		return nil
	}
	out := make([]int64, len(hits))
	var given int64
	top := 0
	for i, h := range hits {
		out[i] = int64(math.Floor(float64(loss) * h.Contribution / total))
		given += out[i]
		c, t := h.Contribution, hits[top].Contribution
		if c > t || (c == t && h.ID < hits[top].ID) {
			top = i
		}
	}
	out[top] += loss - given
	return out
}

// eachFeatureLoss обходит доли потери клиента по причинам; first — первая доля клиента по этой причине.
func eachFeatureLoss(c Customer, fn func(id string, loss int64, first bool)) {
	var buf [8]string
	seen := buf[:0]
	for _, sr := range c.Services {
		for i, v := range AllocateLoss(sr.LossCents, sr.Features) {
			if v == 0 {
				continue
			}
			id := sr.Features[i].ID
			first := !slices.Contains(seen, id)
			if first {
				seen = append(seen, id)
			}
			fn(id, v, first)
		}
	}
}

// lossByFeature суммирует распределение потери по живым услугам всех клиентов; порядок — по убыванию потери, затем по ID.
func lossByFeature(customers []Customer) []FeatureLoss {
	sums := map[string]*FeatureLoss{}
	for _, c := range customers {
		addFeatureLoss(sums, c, 1)
	}
	return sortedFeatureLoss(sums)
}

// addFeatureLoss прибавляет (sign = 1) или вычитает (sign = -1) вклад клиента; причина без клиентов исчезает.
func addFeatureLoss(sums map[string]*FeatureLoss, c Customer, sign int) {
	eachFeatureLoss(c, func(id string, v int64, first bool) {
		fl := sums[id]
		if fl == nil {
			fl = &FeatureLoss{ID: id}
			sums[id] = fl
		}
		fl.LossCents += int64(sign) * v
		if first {
			fl.Customers += sign
		}
		if fl.Customers == 0 {
			delete(sums, id)
		}
	})
}

func sortedFeatureLoss(sums map[string]*FeatureLoss) []FeatureLoss {
	out := make([]FeatureLoss, 0, len(sums))
	for _, fl := range sums {
		out = append(out, *fl)
	}
	slices.SortFunc(out, func(a, b FeatureLoss) int {
		if a.LossCents != b.LossCents {
			return cmp.Compare(b.LossCents, a.LossCents)
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return out
}

package backtest

import (
	"math"
	"sort"
)

// ROCAUC — площадь под ROC по рангам (Манн — Уитни); равные скоры получают средний ранг.
// NaN, если нет положительных или нет отрицательных исходов.
func ROCAUC(scores []float64, pos []bool) float64 {
	n := len(scores)
	ord := make([]int, n)
	for i := range ord {
		ord[i] = i
	}
	sort.SliceStable(ord, func(a, b int) bool { return scores[ord[a]] < scores[ord[b]] })
	var np, sumRank float64
	for i := 0; i < n; {
		j := i
		for j < n && scores[ord[j]] == scores[ord[i]] {
			j++
		}
		rank := float64(i+j+1) / 2 // средний ранг группы, ранги с 1
		for k := i; k < j; k++ {
			if pos[ord[k]] {
				np++
				sumRank += rank
			}
		}
		i = j
	}
	nn := float64(n) - np
	if np == 0 || nn == 0 {
		return math.NaN()
	}
	return (sumRank - np*(np+1)/2) / (np * nn)
}

// PRAUC — средняя точность (площадь под PR по шагам); равные скоры входят одним шагом.
// NaN, если положительных исходов нет.
func PRAUC(scores []float64, pos []bool) float64 {
	n := len(scores)
	ord := make([]int, n)
	var total float64
	for i := range ord {
		ord[i] = i
		if pos[i] {
			total++
		}
	}
	if total == 0 {
		return math.NaN()
	}
	sort.SliceStable(ord, func(a, b int) bool { return scores[ord[a]] > scores[ord[b]] })
	var tp, seen, ap, prevRecall float64
	for i := 0; i < n; {
		j := i
		for j < n && scores[ord[j]] == scores[ord[i]] {
			if pos[ord[j]] {
				tp++
			}
			j++
		}
		seen = float64(j)
		recall := tp / total
		ap += (recall - prevRecall) * tp / seen
		prevRecall = recall
		i = j
	}
	return ap
}

// TopShare — доля потерянных денег у первых ceil(frac·n) элементов порядка ord;
// lost[i] — потерянные деньги элемента i. NaN, если потерь нет.
func TopShare(ord []int, lost []int64, frac float64) float64 {
	var total int64
	for _, v := range lost {
		total += v
	}
	if total == 0 {
		return math.NaN()
	}
	k := int(math.Ceil(frac * float64(len(ord))))
	var top int64
	for _, i := range ord[:k] {
		top += lost[i]
	}
	return float64(top) / float64(total)
}

// Order — индексы по убыванию ключа, при равенстве по возрастанию tie (стабильный порядок).
func Order(key []float64, tie []int64) []int {
	ord := make([]int, len(key))
	for i := range ord {
		ord[i] = i
	}
	sort.SliceStable(ord, func(a, b int) bool {
		x, y := ord[a], ord[b]
		if key[x] != key[y] {
			return key[x] > key[y]
		}
		return tie[x] < tie[y]
	})
	return ord
}

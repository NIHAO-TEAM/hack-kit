package backtest

import (
	"fmt"
	"io"
	"math"
	"strings"
)

func pct(v float64) string {
	if math.IsNaN(v) {
		return "—"
	}
	return fmt.Sprintf("%.1f %%", v*100)
}

func num(v float64) string {
	if math.IsNaN(v) {
		return "—"
	}
	return fmt.Sprintf("%.3f", v)
}

// WriteResult печатает таблицу метрик одной прокрутки markdown-ом.
func WriteResult(w io.Writer, name string, r Result) {
	fmt.Fprintf(w, "### %s\n\n", name)
	fmt.Fprintf(w, "Клиентов с живыми услугами: %d, услуг: %d. Ушло услуг: %d (%s). Потеряно денег: %d из %d ¢ (%s).\n\n",
		r.Customers, r.Services, r.LostServices, pct(ratio(int64(r.LostServices), int64(r.Services))),
		r.LostCents, r.RCents, pct(ratio(r.LostCents, r.RCents)))
	fmt.Fprintln(w, "| Способ | Деньги в верхних 10 % | Деньги в верхних 20 % | ROC AUC | PR AUC |")
	fmt.Fprintln(w, "|---|---|---|---|---|")
	for _, m := range Methods {
		x := r.ByMethod[m]
		fmt.Fprintf(w, "| %s | %s | %s | %s | %s |\n", m, pct(x.Top10), pct(x.Top20), num(x.ROC), num(x.PR))
	}
	fmt.Fprintln(w)
}

// WriteRobustness печатает сдвиги: лучший из правил и сохраняется ли преимущество модели.
func WriteRobustness(w io.Writer, shifts []Shift) {
	fmt.Fprintln(w, "| Сдвиг | Модель: 10 % / 20 % / ROC / PR | Лучшее правило по ROC | Модель выше всех правил по 20 % и ROC |")
	fmt.Fprintln(w, "|---|---|---|---|")
	for _, s := range shifts {
		name := s.Param
		if name == "" {
			name = "все вероятности"
		}
		m := s.Result.ByMethod[Model]
		rn, rv := BestRule(s.Result, func(x Metrics) float64 { return x.ROC })
		_, t20 := BestRule(s.Result, func(x Metrics) float64 { return x.Top20 })
		keep := "нет"
		if m.Top20 > t20 && m.ROC > rv {
			keep = "да"
		}
		fmt.Fprintf(w, "| %s %+.0f %% | %s / %s / %s / %s | %s %s | %s |\n", name, (s.Factor-1)*100,
			pct(m.Top10), pct(m.Top20), num(m.ROC), num(m.PR), rn, num(rv), keep)
	}
	fmt.Fprintln(w)
}

func ratio(a, b int64) float64 {
	if b == 0 {
		return math.NaN()
	}
	return float64(a) / float64(b)
}

// Section — заголовок раздела отчёта.
func Section(w io.Writer, title string) { fmt.Fprintf(w, "## %s\n\n", strings.TrimSpace(title)) }

package risk

import "fmt"

type fail struct{}

func init() { Register(fail{}) }

func (fail) ID() string             { return "fail" }
func (fail) DefaultWeight() float64 { return 0.40 }

// Eval: доля несостоявшихся продлений услуги; продлений нет — 0 (правило 5).
func (fail) Eval(ctx ServiceCtx) (float64, string, bool) {
	total, bad := 0, 0
	for _, t := range ctx.Transactions {
		if t.Type != "prolong" {
			continue
		}
		total++
		if t.Status == "not_performed" {
			bad++
		}
	}
	if total == 0 || bad == 0 {
		return 0, "", true
	}
	return float64(bad) / float64(total), fmt.Sprintf("%d из %d продлений не прошли", bad, total), true
}

package risk

import (
	"fmt"
	"math"
)

type noauto struct{}

func init() { Register(noauto{}) }

func (noauto) ID() string             { return "noauto" }
func (noauto) DefaultWeight() float64 { return 0.55 }

// Eval: автопродление выключено, f растёт от 0 до 1 за 7 дней до конца срока (правило 5).
func (noauto) Eval(ctx ServiceCtx) (float64, string, bool) {
	if ctx.Service.AutoProlong {
		return 0, "", false
	}
	days := ctx.Service.ExpiresAt.Sub(ctx.AsOf).Seconds() / 86400
	if days > 7 {
		return 0, "", true
	}
	f := math.Min(1, 1-days/7)
	if ctx.NoReason {
		return f, "", true
	}
	if days <= 0 {
		return f, "Автопродление выключено, срок услуги уже вышел", true
	}
	return f, fmt.Sprintf("Автопродление выключено, срок услуги закончится через %s", daysText(days)), true
}

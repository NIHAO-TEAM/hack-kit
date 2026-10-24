package risk

import (
	"math"
	"time"
)

// pwaitGrace — сколько дней услуга ждёт оплаты до удаления.
const pwaitGrace = 5 * 24 * time.Hour

type pwait struct{}

func init() { Register(pwait{}) }

func (pwait) ID() string             { return "pwait" }
func (pwait) DefaultWeight() float64 { return 0.80 }

// Eval: ожидание оплаты, f растёт от 0 до 1 за 5 дней до удаления (правило 5).
func (pwait) Eval(ctx ServiceCtx) (float64, string, bool) {
	if ctx.Service.Status != StatusProlongWait {
		return 0, "", false
	}
	left := ctx.Service.ExpiresAt.Add(pwaitGrace).Sub(ctx.AsOf)
	if left <= 0 {
		return 1, "Срок ожидания оплаты вышел: услугу могут удалить в любой момент", true
	}
	days := left.Seconds() / 86400
	f := math.Min(1, math.Max(0, 1-days/5))
	if ctx.NoReason {
		return f, "", true
	}
	return f, "Услуга ждёт оплаты, до удаления осталось " + daysText(days), true
}

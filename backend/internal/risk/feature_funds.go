package risk

import "fmt"

type funds struct{}

func init() { Register(funds{}) }

func (funds) ID() string             { return "funds" }
func (funds) DefaultWeight() float64 { return 0.70 }

// Eval: не хватает средств на продления ближайших 7 дней, f — доля нехватки (правило 5).
// «Нужно» и «есть» ядро считает один раз на аккаунт (ctx.Funds).
func (funds) Eval(ctx ServiceCtx) (float64, string, bool) {
	need, have := ctx.Funds.Need, ctx.Funds.Have
	if need <= 0 || have >= need {
		return 0, "", true
	}
	f := float64(need-have) / float64(need)
	if ctx.NoReason {
		return f, "", true
	}
	return f, fmt.Sprintf("Нужно %s на 7 дней, на балансе %s", euros(need), euros(have)), true
}

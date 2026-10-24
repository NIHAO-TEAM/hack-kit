package risk

import "fmt"

type funds struct{}

func init() { Register(funds{}) }

func (funds) ID() string             { return "funds" }
func (funds) DefaultWeight() float64 { return 0.70 }

// Eval: не хватает средств на продления ближайших 7 дней, f — доля нехватки (правило 5).
// «Нужно» и «есть» ядро считает один раз на аккаунт (ctx.Funds), но признак есть только у услуг,
// у которых в окне запаса средств есть списание: prolong_wait или списания(168 ч) > 0.
func (funds) Eval(ctx ServiceCtx) (float64, string, bool) {
	if charges(ctx.Service, ctx.AsOf, fundsHours) == 0 {
		return 0, "", false
	}
	need, have := ctx.Funds.Need, ctx.Funds.Have
	if need <= 0 || have >= need {
		return 0, "", true
	}
	f := float64(need-have) / float64(need)
	return f, fmt.Sprintf("Нужно %s на 7 дней, на балансе %s", euros(need), euros(have)), true
}

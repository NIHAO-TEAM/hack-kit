package risk

import "fmt"

type flight struct{}

func init() { Register(flight{}) }

func (flight) ID() string             { return "flight" }
func (flight) DefaultWeight() float64 { return 0.15 }

// Eval: доля баланса, доступного к выводу; баланс 0 — 0 (правило 5).
func (flight) Eval(ctx ServiceCtx) (float64, string, bool) {
	a := ctx.Account
	if a.Balance == 0 {
		return 0, "", true
	}
	f := float64(a.WithdrawBalance) / float64(a.Balance)
	if f <= 0 {
		return 0, "", true
	}
	return f, fmt.Sprintf("К выводу доступно %s из %s на балансе", euros(a.WithdrawBalance), euros(a.Balance)), true
}

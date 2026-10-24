package risk

import "fmt"

type flight struct{}

func init() { Register(flight{}) }

func (flight) ID() string { return "flight" }

// DefaultWeight 0: withdrawBalance — реферальный баланс, доступный к выводу; факта вывода в данных нет.
// Признак считается и виден в карточке, на скор влияет после включения веса на экране «Модель».
func (flight) DefaultWeight() float64 { return 0 }

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
	if ctx.NoReason {
		return f, "", true
	}
	return f, fmt.Sprintf("Доступно к выводу %s из %s. Факт вывода в данных не виден; по умолчанию не влияет на риск", euros(a.WithdrawBalance), euros(a.Balance)), true
}

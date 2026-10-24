package risk

import "fmt"

type bonusLocked struct{}

func init() { Register(bonusLocked{}) }

func (bonusLocked) ID() string { return "bonus_locked" }

// DefaultWeight 0: признак считается и виден в карточке, на скор влияет после включения веса на экране «Модель».
func (bonusLocked) DefaultWeight() float64 { return 0 }

// Eval: на балансе есть заблокированный бонус, а средств на 7 дней не хватает — 1; иначе признак не применим.
func (bonusLocked) Eval(ctx ServiceCtx) (float64, string, bool) {
	a := ctx.Account
	if a.BonusBalance <= 0 || a.BonusState != "locked" || ctx.Funds.Have >= ctx.Funds.Need {
		return 0, "", false
	}
	if ctx.NoReason {
		return 1, "", true
	}
	return 1, fmt.Sprintf("Бонус %s заблокирован, средств на 7 дней не хватает", euros(a.BonusBalance)), true
}

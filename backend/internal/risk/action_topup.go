package risk

import "fmt"

type topUp struct{}

func init() { RegisterAction(topUp{}) }

func (topUp) ID() string { return "top_up" }

// Applies: на окно запаса средств не хватает денег (правило 5, funds > 0).
func (topUp) Applies(ctx CustomerCtx) bool { return ctx.Funds.Need > ctx.Funds.Have }

// CostCents — сумма пополнения.
func (topUp) CostCents(ctx CustomerCtx) int64 { return ctx.Funds.Need - ctx.Funds.Have }

func (topUp) Title(ctx CustomerCtx) string {
	return fmt.Sprintf("Пополнить баланс на %s", euros(ctx.Funds.Need-ctx.Funds.Have))
}

// Apply без настроек считает «нужно» по умолчанию; ядро вызывает ApplyWith с настройками расчёта.
func (a topUp) Apply(s Snapshot, accountID int64) Snapshot {
	return a.ApplyWith(s, accountID, Defaults())
}

// ApplyWith добавляет к балансу аккаунта «нужно − есть» (центы, целые).
func (topUp) ApplyWith(s Snapshot, accountID int64, st Settings) Snapshot {
	out := s
	out.Accounts = append([]Account(nil), s.Accounts...)
	for i, a := range out.Accounts {
		if a.ID != accountID {
			continue
		}
		f := accountFunds(a, liveServices(s, accountID), s.AsOf, st)
		if f.Need > f.Have {
			out.Accounts[i].Balance += f.Need - f.Have
		}
	}
	return out
}

func liveServices(s Snapshot, accountID int64) []Service {
	var live []Service
	for _, sv := range s.Services {
		if sv.OwnerID == accountID && sv.IsLive() {
			live = append(live, sv)
		}
	}
	return live
}

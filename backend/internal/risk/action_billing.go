package risk

import "fmt"

type toBilling struct{}

func init() { RegisterAction(toBilling{}) }

func (toBilling) ID() string { return "to_billing" }

const (
	txProlong      = "prolong"
	txNotPerformed = "not_performed"
)

// failedProlongs — сколько непрошедших продлений у услуг, подпадающих под сбой автосписания:
// услуга в ожидании продления, автопродление включено, у неё есть транзакция prolong в not_performed.
func failedProlongs(ctx CustomerCtx) int {
	waiting := map[int64]bool{}
	for _, sv := range ctx.Services {
		if sv.AutoProlong && sv.Status == StatusProlongWait {
			waiting[sv.ID] = true
		}
	}
	n := 0
	for _, t := range ctx.Transactions {
		if waiting[t.ServiceID] && t.Type == txProlong && t.Status == txNotPerformed {
			n++
		}
	}
	return n
}

// Applies: сбой списания — денег хватает, услуга ждёт продления с включённым автопродлением, и попытка
// продления не прошла (транзакция prolong в not_performed). Deleted/blocked — не биллинг.
func (toBilling) Applies(ctx CustomerCtx) bool {
	if ctx.Funds.Have < ctx.Funds.Need {
		return false
	}
	return failedProlongs(ctx) > 0
}

func (toBilling) Title(ctx CustomerCtx) string {
	return fmt.Sprintf("К биллингу: %d продлений не прошли при достаточном балансе", failedProlongs(ctx))
}

// Apply не меняет данные: эффект 0.
func (toBilling) Apply(s Snapshot, _ int64) Snapshot { return s }

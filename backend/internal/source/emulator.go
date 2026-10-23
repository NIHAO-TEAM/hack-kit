package source

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"example.com/app/internal/platform/api/gen"
)

// Действия эмулятора (actionId в EmulatorAction).
const (
	ActionTopUp             = "top_up"
	ActionEnableAutoprolong = "enable_autoprolong"
	ActionReset             = "reset"
)

// prolongWaitLimit — сколько услуга живёт в prolong_wait до удаления.
const prolongWaitLimit = 5 * 24 * time.Hour

// termDuration — длина оплаченного срока.
func termDuration(term string) time.Duration {
	switch term {
	case "hour":
		return time.Hour
	case "week":
		return 7 * 24 * time.Hour
	case "month":
		return 30 * 24 * time.Hour
	case "quarter_year":
		return 90 * 24 * time.Hour
	case "half_year":
		return 180 * 24 * time.Hour
	case "year":
		return 365 * 24 * time.Hour
	}
	return 30 * 24 * time.Hour
}

// Advance двигает снимок на hours часов и возвращает новый; исходный не меняется.
// Активная услуга с истёкшим сроком продлевается и списывает цену с баланса владельца,
// если включено автопродление и денег хватает (почасовая — каждый час); иначе уходит
// в prolong_wait. prolong_wait старше 5 дней от конца срока становится deleted.
// Чистая функция: часов и ввода-вывода нет.
func Advance(s Snapshot, hours int) Snapshot {
	asOf := time.Time{}
	if s.AsOf != nil {
		asOf = s.AsOf.UTC()
	}
	end := asOf.Add(time.Duration(hours) * time.Hour)

	out := Snapshot{AsOf: &end}
	out.Accounts = append([]Account(nil), s.Accounts...)
	out.Services = append([]Service(nil), s.Services...)
	out.Transactions = append([]Transaction(nil), s.Transactions...)

	byID := make(map[int64]*Account, len(out.Accounts))
	for i := range out.Accounts {
		byID[out.Accounts[i].ID] = &out.Accounts[i]
	}
	for i := range out.Services {
		v := &out.Services[i]
		if v.Status == "active" {
			step := termDuration(v.PaymentTerm)
			for v.Status == "active" && !v.ExpiresAt.After(end) {
				acc := byID[v.OwnerID]
				if !v.AutoProlong || acc == nil || acc.Balance < v.Price {
					v.Status = "prolong_wait"
					break
				}
				acc.Balance -= v.Price
				acc.WithdrawBalance += v.Price
				v.ExpiresAt = v.ExpiresAt.Add(step)
			}
		}
		if v.Status == "prolong_wait" && !v.ExpiresAt.Add(prolongWaitLimit).After(end) {
			v.Status = "deleted"
		}
	}
	return out
}

// applyAction применяет одно действие к снимку на месте. reset сюда не приходит.
func applyAction(s *Snapshot, a gen.EmulatorAction) error {
	switch a.ActionId {
	case ActionTopUp:
		if a.Cents == nil || *a.Cents <= 0 {
			return errors.New("top_up: нужна сумма cents больше нуля")
		}
		for i := range s.Accounts {
			if s.Accounts[i].ID == a.CustomerId {
				s.Accounts[i].Balance += *a.Cents
				s.Accounts[i].TotalReplenished += *a.Cents
				return nil
			}
		}
	case ActionEnableAutoprolong:
		found := false
		for i := range s.Services {
			if s.Services[i].OwnerID == a.CustomerId {
				found = true
				if s.Services[i].Status != "deleted" {
					s.Services[i].AutoProlong = true
				}
			}
		}
		if found {
			return nil
		}
	default:
		return fmt.Errorf("неизвестное действие %q", a.ActionId)
	}
	return fmt.Errorf("%s: клиент %d не найден", a.ActionId, a.CustomerId)
}

// resetSnapshot — снимок сида: организаторы и, при SEED_STAND=true, стенд.
func resetSnapshot() (*Snapshot, error) {
	body, err := seedBody()
	if err != nil {
		return nil, err
	}
	s, errs, err := ParseSnapshot(bytes.NewReader(body))
	if err != nil || len(errs) > 0 {
		return nil, fmt.Errorf("source: вшитый сид не разбирается: %v %v", err, errs)
	}
	return s, nil
}

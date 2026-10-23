package source

import (
	"fmt"

	"example.com/app/internal/platform/api/gen"
)

// patchService применяет правку к услуге на месте и проверяет снимок целиком.
func patchService(s *Snapshot, id int64, p gen.ServicePatch) (Service, error) {
	for i := range s.Services {
		v := &s.Services[i]
		if v.ID != id {
			continue
		}
		if p.Status != nil {
			v.Status = string(*p.Status)
		}
		if p.AutoProlong != nil {
			v.AutoProlong = *p.AutoProlong
		}
		if p.ExpiresAt != nil {
			v.ExpiresAt = p.ExpiresAt.UTC()
		}
		return *v, checkPatched(s)
	}
	return Service{}, notFound{fmt.Errorf("услуга %d не найдена", id)}
}

// patchAccount применяет правку к счёту на месте и проверяет снимок целиком.
func patchAccount(s *Snapshot, id int64, p gen.AccountPatch) (Account, error) {
	for i := range s.Accounts {
		a := &s.Accounts[i]
		if a.ID != id {
			continue
		}
		if p.Balance != nil {
			a.Balance = *p.Balance
		}
		if p.BonusBalance != nil {
			a.BonusBalance = *p.BonusBalance
		}
		return *a, checkPatched(s)
	}
	return Account{}, notFound{fmt.Errorf("счёт %d не найден", id)}
}

// checkPatched отклоняет правку, после которой снимок не проходит проверку (неизвестный статус,
// отрицательная сумма): данные остаются прежними.
func checkPatched(s *Snapshot) error {
	if errs := Validate(s); len(errs) > 0 {
		return badRequest{fmt.Errorf("%s: %s", errs[0].Field, errs[0].Message)}
	}
	return nil
}

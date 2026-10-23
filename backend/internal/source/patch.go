package source

import (
	"encoding/json"
	"fmt"
	"net/http"

	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/httpx"
)

// PatchService — PATCH /api/v1/source/services/{id}. Меняет одну услугу и пишет новый снимок
// тем же путём, что PUT: одна транзакция, событие source.changed.
func (a *SourceAPI) PatchService(w http.ResponseWriter, r *http.Request, id gen.ServiceId) {
	var req gen.ServicePatch
	if !decodeBody(w, r, &req) {
		return
	}
	var out Service
	_, _, ok := a.commit(w, r, func(s *Snapshot) (*Snapshot, string, error) {
		v, err := patchService(s, int64(id), req)
		out = v
		return s, OriginAPI, err
	})
	if !ok {
		return
	}
	writeJSON(w, gen.Service{
		Id: out.ID, Name: out.Name, OwnerId: out.OwnerID, PaymentTerm: gen.PaymentTerm(out.PaymentTerm),
		AutoProlong: out.AutoProlong, CreatedAt: out.CreatedAt, ExpiresAt: out.ExpiresAt,
		Status: gen.ServiceStatus(out.Status), Price: out.Price,
	})
}

// PatchAccount — PATCH /api/v1/source/accounts/{id}.
func (a *SourceAPI) PatchAccount(w http.ResponseWriter, r *http.Request, id gen.AccountId) {
	var req gen.AccountPatch
	if !decodeBody(w, r, &req) {
		return
	}
	var out Account
	_, _, ok := a.commit(w, r, func(s *Snapshot) (*Snapshot, string, error) {
		v, err := patchAccount(s, int64(id), req)
		out = v
		return s, OriginAPI, err
	})
	if !ok {
		return
	}
	writeJSON(w, gen.Account{
		Id: out.ID, Email: out.Email, Balance: out.Balance, WithdrawBalance: out.WithdrawBalance,
		TotalReplenished: out.TotalReplenished, BonusBalance: out.BonusBalance,
		BonusState: gen.AccountBonusState(out.BonusState),
	})
}

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

// decodeBody разбирает JSON-тело правки; при ошибке отвечает 400 и возвращает false.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(v); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "тело не разобрано: "+err.Error())
		return false
	}
	return true
}

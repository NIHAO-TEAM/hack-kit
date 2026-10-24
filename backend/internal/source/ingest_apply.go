package source

import (
	"errors"
	"fmt"
	"time"
)

// ingestState — что нужно для проверки события без чтения базы: какие счета и услуги есть и когда
// создана услуга. Строится один раз из снимка прогона. Событие этих полей не меняет, поэтому
// состояние остаётся верным весь прогон и его не нужно править после каждого события.
// Весь снимок событие не проверяет: правка одной строки не может испортить остальные.
type ingestState struct {
	accounts map[int64]struct{}
	created  map[int64]time.Time // номер услуги → createdAt
	owner    map[int64]int64     // номер услуги → владелец
}

// newOpenIngestState — состояние без списка счетов и услуг: проверяются только значения, а
// существование строки решает запрос записи (ни одной затронутой строки — отказ). Для приёма
// извне, где снимок может смениться между проверкой и записью.
func newOpenIngestState() *ingestState { return &ingestState{} }

func newIngestState(s *Snapshot) *ingestState {
	st := &ingestState{
		accounts: make(map[int64]struct{}, len(s.Accounts)),
		created:  make(map[int64]time.Time, len(s.Services)),
		owner:    make(map[int64]int64, len(s.Services)),
	}
	for _, a := range s.Accounts {
		st.accounts[a.ID] = struct{}{}
	}
	for _, v := range s.Services {
		st.created[v.ID] = v.CreatedAt
		st.owner[v.ID] = v.OwnerID
	}
	return st
}

// accountsOf — клиенты, чьи строки правит событие: владелец счёта и владелец услуги.
func (st *ingestState) accountsOf(ev StreamEvent) []int64 {
	var out []int64
	if ev.Balance != nil {
		out = append(out, ev.AccountID)
	}
	if ev.Status != nil || ev.AutoProlong != nil || ev.ExpiresAt != nil {
		out = append(out, st.owner[ev.ServiceID])
	}
	return out
}

// check проверяет событие теми же правилами, что patchAccount, patchService и Validate: счёт и
// услуга существуют, сумма в пределах, статус известен, срок не раньше создания.
func (st *ingestState) check(ev StreamEvent) error {
	open := st.accounts == nil
	if ev.Balance != nil {
		if _, ok := st.accounts[ev.AccountID]; !ok && !open {
			return notFound{fmt.Errorf("счёт %d не найден", ev.AccountID)}
		}
		if *ev.Balance < 0 {
			return badRequest{fmt.Errorf("accounts.balance: отрицательная сумма %d", *ev.Balance)}
		}
		if *ev.Balance > maxMoneyCents {
			return badRequest{fmt.Errorf("accounts.balance: сумма %d больше предела %d", *ev.Balance, maxMoneyCents)}
		}
	}
	if ev.Status != nil || ev.AutoProlong != nil || ev.ExpiresAt != nil {
		created, ok := st.created[ev.ServiceID]
		if !ok && !open {
			return notFound{fmt.Errorf("услуга %d не найдена", ev.ServiceID)}
		}
		if ev.Status != nil && !serviceStates[*ev.Status] {
			return badRequest{fmt.Errorf("services.status: неизвестный статус %q", *ev.Status)}
		}
		if ev.ExpiresAt != nil && ok && ev.ExpiresAt.UTC().Before(created) {
			return badRequest{errors.New("services.expiresAt: раньше createdAt")}
		}
	}
	return nil
}

package source

import (
	"fmt"
	"regexp"
	"time"
)

// FieldError — ошибка одного поля снимка; путь вида services[3].ownerId.
type FieldError struct {
	Field   string
	Message string
}

// maxMoneyCents — предел любой суммы снимка: 10 млн евро в центах. Выше платежи на горизонт
// (цена × число списаний) по всем услугам переполняют int64 и дают отрицательные потери.
const maxMoneyCents = 1_000_000_000

// maxErrors — сколько ошибок возвращается в errors[].
const maxErrors = 100

var serviceName = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

var (
	paymentTerms  = set("hour", "week", "month", "quarter_year", "half_year", "year")
	serviceStates = set("active", "blocked", "deleted", "suspended", "prolong_wait")
	bonusStates   = set("locked", "unlocked")
	txStates      = set("not_performed", "performed")
	txTypes       = set("prolong", "order")
)

func set(v ...string) map[string]bool {
	m := make(map[string]bool, len(v))
	for _, s := range v {
		m[s] = true
	}
	return m
}

type errList struct{ list []FieldError }

func (e *errList) add(path, format string, args ...any) {
	e.list = append(e.list, FieldError{Field: path, Message: fmt.Sprintf(format, args...)})
}

// Validate проверяет связи и значения снимка до записи. Возвращает не более maxErrors ошибок.
// Чистая функция: ввода-вывода нет.
func Validate(s *Snapshot) []FieldError {
	e := &errList{}
	accounts := make(map[int64]bool, len(s.Accounts))
	emails := make(map[string]bool, len(s.Accounts))
	for i, a := range s.Accounts {
		p := fmt.Sprintf("accounts[%d]", i)
		if accounts[a.ID] {
			e.add(p+".id", "повтор id %d", a.ID)
		}
		accounts[a.ID] = true
		if a.Email == "" {
			e.add(p+".email", "пустой email")
		} else if emails[a.Email] {
			e.add(p+".email", "повтор email %q", a.Email)
		}
		emails[a.Email] = true
		for name, v := range map[string]int64{
			"balance": a.Balance, "withdrawBalance": a.WithdrawBalance,
			"totalReplenished": a.TotalReplenished, "bonusBalance": a.BonusBalance,
		} {
			if v < 0 {
				e.add(p+"."+name, "отрицательная сумма %d", v)
			} else if v > maxMoneyCents {
				e.add(p+"."+name, "сумма %d больше предела %d", v, maxMoneyCents)
			}
		}
		if !bonusStates[a.BonusState] {
			e.add(p+".bonusState", "неизвестное значение %q", a.BonusState)
		}
	}
	services := make(map[int64]bool, len(s.Services))
	for i, v := range s.Services {
		p := fmt.Sprintf("services[%d]", i)
		if services[v.ID] {
			e.add(p+".id", "повтор id %d", v.ID)
		}
		services[v.ID] = true
		if !serviceName.MatchString(v.Name) {
			e.add(p+".name", "допустимы латиница, цифры и дефис")
		}
		if !accounts[v.OwnerID] {
			e.add(p+".ownerId", "аккаунт %d не найден", v.OwnerID)
		}
		if !paymentTerms[v.PaymentTerm] {
			e.add(p+".paymentTerm", "неизвестное значение %q", v.PaymentTerm)
		}
		if !serviceStates[v.Status] {
			e.add(p+".status", "неизвестный статус %q", v.Status)
		}
		if v.Price < 0 {
			e.add(p+".price", "отрицательная сумма %d", v.Price)
		} else if v.Price > maxMoneyCents {
			e.add(p+".price", "сумма %d больше предела %d", v.Price, maxMoneyCents)
		}
		if v.ExpiresAt.Before(v.CreatedAt) {
			e.add(p+".expiresAt", "раньше createdAt")
		}
	}
	txs := make(map[int64]bool, len(s.Transactions))
	for i, t := range s.Transactions {
		p := fmt.Sprintf("transactions[%d]", i)
		if txs[t.ID] {
			e.add(p+".id", "повтор id %d", t.ID)
		}
		txs[t.ID] = true
		if t.Amount < 0 || t.Amount > 1000 {
			e.add(p+".amount", "сумма %d вне 0..1000", t.Amount)
		}
		if !txStates[t.Status] {
			e.add(p+".status", "неизвестный статус %q", t.Status)
		}
		if !txTypes[t.Type] {
			e.add(p+".type", "неизвестное значение %q", t.Type)
		}
		if !services[t.ServiceID] {
			e.add(p+".serviceId", "услуга %d не найдена", t.ServiceID)
		}
		switch {
		case t.Status == "not_performed" && t.PerformedAt != nil:
			e.add(p+".performedAt", "у not_performed даты исполнения быть не должно")
		case t.Status == "performed" && t.PerformedAt == nil:
			e.add(p+".performedAt", "у performed дата исполнения обязательна")
		case t.PerformedAt != nil && t.PerformedAt.Before(t.CreatedAt):
			e.add(p+".performedAt", "раньше createdAt")
		}
	}
	if len(e.list) > maxErrors {
		e.list = e.list[:maxErrors]
	}
	return e.list
}

// ModelTime — время модели: asOf из тела; без него — самая поздняя дата события снимка
// (services.createdAt, transactions.createdAt, transactions.performedAt); у пустого снимка
// без asOf — время приёма now.
func ModelTime(s *Snapshot, now time.Time) time.Time {
	if s.AsOf != nil {
		return s.AsOf.UTC()
	}
	var latest time.Time
	bump := func(t time.Time) {
		if t.After(latest) {
			latest = t
		}
	}
	for _, v := range s.Services {
		bump(v.CreatedAt)
	}
	for _, t := range s.Transactions {
		bump(t.CreatedAt)
		if t.PerformedAt != nil {
			bump(*t.PerformedAt)
		}
	}
	if latest.IsZero() {
		return now.UTC()
	}
	return latest.UTC()
}

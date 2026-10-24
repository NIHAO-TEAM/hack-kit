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

// add копит не более maxErrors ошибок: остальные всё равно не вернутся.
func (e *errList) add(path, format string, args ...any) {
	if len(e.list) >= maxErrors {
		return
	}
	e.list = append(e.list, FieldError{Field: path, Message: fmt.Sprintf(format, args...)})
}

// ref — ссылка записи на запись другой таблицы, которую ещё нельзя проверить.
type ref struct {
	index int
	id    int64
}

// validator проверяет записи по одной: в памяти только множества id и email, а не сами записи.
// Ссылку (услуга → владелец, платёж → услуга) проверяет сразу, если массив назначения уже закончился;
// иначе откладывает до finish: массивы в теле идут в любом порядке.
type validator struct {
	e        errList
	accounts idSet
	emails   map[string]struct{}
	services idSet
	txs      idSet
	done     [3]bool
	owners   []ref // услуги, чьи аккаунты ещё не закончились
	svcRefs  []ref // платежи, чьи услуги ещё не закончились
}

func newValidator(accounts int) *validator {
	return &validator{emails: make(map[string]struct{}, accounts)}
}

func (v *validator) account(i int, a Account) {
	p := fmt.Sprintf("accounts[%d]", i)
	if v.accounts.add(a.ID) {
		v.e.add(p+".id", "повтор id %d", a.ID)
	}
	if a.Email == "" {
		v.e.add(p+".email", "пустой email")
	} else if _, dup := v.emails[a.Email]; dup {
		v.e.add(p+".email", "повтор email %q", a.Email)
	}
	v.emails[a.Email] = struct{}{}
	for _, m := range [...]struct {
		name string
		v    int64
	}{
		{"balance", a.Balance}, {"withdrawBalance", a.WithdrawBalance},
		{"totalReplenished", a.TotalReplenished}, {"bonusBalance", a.BonusBalance},
	} {
		if m.v < 0 {
			v.e.add(p+"."+m.name, "отрицательная сумма %d", m.v)
		} else if m.v > maxMoneyCents {
			v.e.add(p+"."+m.name, "сумма %d больше предела %d", m.v, maxMoneyCents)
		}
	}
	if !bonusStates[a.BonusState] {
		v.e.add(p+".bonusState", "неизвестное значение %q", a.BonusState)
	}
}

func (v *validator) service(i int, s Service) {
	p := fmt.Sprintf("services[%d]", i)
	if v.services.add(s.ID) {
		v.e.add(p+".id", "повтор id %d", s.ID)
	}
	if !serviceName.MatchString(s.Name) {
		v.e.add(p+".name", "допустимы латиница, цифры и дефис")
	}
	if v.done[tableAccounts] {
		if !v.accounts.has(s.OwnerID) {
			v.e.add(p+".ownerId", "аккаунт %d не найден", s.OwnerID)
		}
	} else {
		v.owners = append(v.owners, ref{i, s.OwnerID})
	}
	if !paymentTerms[s.PaymentTerm] {
		v.e.add(p+".paymentTerm", "неизвестное значение %q", s.PaymentTerm)
	}
	if !serviceStates[s.Status] {
		v.e.add(p+".status", "неизвестный статус %q", s.Status)
	}
	if s.Price < 0 {
		v.e.add(p+".price", "отрицательная сумма %d", s.Price)
	} else if s.Price > maxMoneyCents {
		v.e.add(p+".price", "сумма %d больше предела %d", s.Price, maxMoneyCents)
	}
	if s.ExpiresAt.Before(s.CreatedAt) {
		v.e.add(p+".expiresAt", "раньше createdAt")
	}
}

func (v *validator) transaction(i int, t Transaction) {
	p := fmt.Sprintf("transactions[%d]", i)
	if v.txs.add(t.ID) {
		v.e.add(p+".id", "повтор id %d", t.ID)
	}
	if t.Amount < 0 || t.Amount > 1000 {
		v.e.add(p+".amount", "сумма %d вне 0..1000", t.Amount)
	}
	if !txStates[t.Status] {
		v.e.add(p+".status", "неизвестный статус %q", t.Status)
	}
	if !txTypes[t.Type] {
		v.e.add(p+".type", "неизвестное значение %q", t.Type)
	}
	if v.done[tableServices] {
		if !v.services.has(t.ServiceID) {
			v.e.add(p+".serviceId", "услуга %d не найдена", t.ServiceID)
		}
	} else {
		v.svcRefs = append(v.svcRefs, ref{i, t.ServiceID})
	}
	switch {
	case t.Status == "not_performed" && t.PerformedAt != nil:
		v.e.add(p+".performedAt", "у not_performed даты исполнения быть не должно")
	case t.Status == "performed" && t.PerformedAt == nil:
		v.e.add(p+".performedAt", "у performed дата исполнения обязательна")
	case t.PerformedAt != nil && t.PerformedAt.Before(t.CreatedAt):
		v.e.add(p+".performedAt", "раньше createdAt")
	}
}

// endTable отмечает конец массива: отложенные ссылки на него проверяются здесь.
func (v *validator) endTable(t table) {
	v.done[t] = true
	switch t {
	case tableAccounts:
		for _, r := range v.owners {
			if !v.accounts.has(r.id) {
				v.e.add(fmt.Sprintf("services[%d].ownerId", r.index), "аккаунт %d не найден", r.id)
			}
		}
		v.owners = nil
	case tableServices:
		for _, r := range v.svcRefs {
			if !v.services.has(r.id) {
				v.e.add(fmt.Sprintf("transactions[%d].serviceId", r.index), "услуга %d не найдена", r.id)
			}
		}
		v.svcRefs = nil
	}
}

// finish проверяет ссылки на массивы, которых в теле не было, и возвращает не более maxErrors ошибок.
func (v *validator) finish() []FieldError {
	for _, t := range [...]table{tableAccounts, tableServices} {
		if !v.done[t] {
			v.endTable(t)
		}
	}
	return v.e.list
}

// Validate проверяет связи и значения снимка до записи. Возвращает не более maxErrors ошибок.
// Чистая функция: ввода-вывода нет.
func Validate(s *Snapshot) []FieldError {
	v := newValidator(len(s.Accounts))
	for i, a := range s.Accounts {
		v.account(i, a)
	}
	v.endTable(tableAccounts)
	for i, x := range s.Services {
		v.service(i, x)
	}
	v.endTable(tableServices)
	for i, x := range s.Transactions {
		v.transaction(i, x)
	}
	return v.finish()
}

// ModelTime — время модели: asOf из тела; без него — самая поздняя дата события снимка
// (services.createdAt, transactions.createdAt, transactions.performedAt); у пустого снимка
// без asOf — время приёма now.
func ModelTime(s *Snapshot, now time.Time) time.Time {
	var l latest
	for _, v := range s.Services {
		l.service(v)
	}
	for _, t := range s.Transactions {
		l.transaction(t)
	}
	return l.modelTime(s.AsOf, now)
}

// latest копит самую позднюю дату события, пока записи идут мимо.
type latest struct{ t time.Time }

func (l *latest) bump(t time.Time) {
	if t.After(l.t) {
		l.t = t
	}
}

func (l *latest) service(v Service) { l.bump(v.CreatedAt) }

func (l *latest) transaction(t Transaction) {
	l.bump(t.CreatedAt)
	if t.PerformedAt != nil {
		l.bump(*t.PerformedAt)
	}
}

func (l *latest) modelTime(asOf *time.Time, now time.Time) time.Time {
	switch {
	case asOf != nil:
		return asOf.UTC()
	case l.t.IsZero():
		return now.UTC()
	}
	return l.t.UTC()
}

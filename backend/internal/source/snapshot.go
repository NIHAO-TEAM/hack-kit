package source

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

// Account, Service и Transaction — записи снимка в именах схемы организаторов.
// Деньги — целые евроценты.
type Account struct {
	ID               int64
	Email            string
	Balance          int64
	WithdrawBalance  int64
	TotalReplenished int64
	BonusBalance     int64
	BonusState       string
}

type Service struct {
	ID          int64
	Name        string
	OwnerID     int64
	PaymentTerm string
	AutoProlong bool
	CreatedAt   time.Time
	ExpiresAt   time.Time
	Status      string
	Price       int64
}

type Transaction struct {
	ID          int64
	Amount      int64
	Status      string
	PerformedAt *time.Time
	CreatedAt   time.Time
	Type        string
	ServiceID   int64
}

// Snapshot — разобранный снимок. AsOf nil, если в теле его нет.
type Snapshot struct {
	AsOf         *time.Time
	Accounts     []Account
	Services     []Service
	Transactions []Transaction
}

// Сырые записи: указатели показывают, что поле передано; Pice и ему подобные — алиасы.
type rawAccount struct {
	ID               *int64  `json:"id"`
	Email            *string `json:"email"`
	Balance          *int64  `json:"balance"`
	WithdrawBalance  *int64  `json:"withdrawBalance"`
	TotalReplenished *int64  `json:"totalReplenished"`
	BonusBalance     *int64  `json:"bonusBalance"`
	BonusState       *string `json:"bonusState"`
}

type rawService struct {
	ID          *int64     `json:"id"`
	Name        *string    `json:"name"`
	OwnerID     *int64     `json:"ownerId"`
	PaymentTerm *string    `json:"paymentTerm"`
	PaymentTem  *string    `json:"paymentTem"`
	AutoProlong *bool      `json:"autoProlong"`
	Autoprolong *bool      `json:"autoprolong"`
	CreatedAt   *time.Time `json:"createdAt"`
	ExpiresAt   *time.Time `json:"expiresAt"`
	Status      *string    `json:"status"`
	Price       *int64     `json:"price"`
	Pice        *int64     `json:"pice"`
}

type rawTransaction struct {
	ID          *int64     `json:"id"`
	Amount      *int64     `json:"amount"`
	Status      *string    `json:"status"`
	PerformedAt *time.Time `json:"performedAt"`
	CreatedAt   *time.Time `json:"createdAt"`
	Type        *string    `json:"type"`
	ServiceID   *int64     `json:"serviceId"`
}

// ParseSnapshot читает тело потоково: массивы разбираются по одной записи, тело целиком
// в памяти не держится. Ошибки формата и обязательных полей собираются в []FieldError;
// ошибка второго результата — тело не JSON и не форма снимка.
func ParseSnapshot(r io.Reader) (*Snapshot, []FieldError, error) {
	dec := json.NewDecoder(r)
	if err := expectDelim(dec, '{'); err != nil {
		return nil, nil, err
	}
	s := &Snapshot{}
	var errs []FieldError
	seen := map[string]bool{}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, nil, err
		}
		key, _ := tok.(string)
		seen[key] = true
		switch key {
		case "asOf":
			var t *time.Time
			if err := dec.Decode(&t); err != nil {
				return nil, nil, fmt.Errorf("asOf: %w", err)
			}
			s.AsOf = t
		case "accounts":
			if err := streamArray(dec, func(i int) error {
				var a rawAccount
				if err := dec.Decode(&a); err != nil {
					return err
				}
				acc, e := a.convert(i)
				s.Accounts = append(s.Accounts, acc)
				errs = append(errs, e...)
				return nil
			}); err != nil {
				return nil, nil, fmt.Errorf("accounts: %w", err)
			}
		case "services":
			if err := streamArray(dec, func(i int) error {
				var v rawService
				if err := dec.Decode(&v); err != nil {
					return err
				}
				svc, e := v.convert(i)
				s.Services = append(s.Services, svc)
				errs = append(errs, e...)
				return nil
			}); err != nil {
				return nil, nil, fmt.Errorf("services: %w", err)
			}
		case "transactions":
			if err := streamArray(dec, func(i int) error {
				var v rawTransaction
				if err := dec.Decode(&v); err != nil {
					return err
				}
				tx, e := v.convert(i)
				s.Transactions = append(s.Transactions, tx)
				errs = append(errs, e...)
				return nil
			}); err != nil {
				return nil, nil, fmt.Errorf("transactions: %w", err)
			}
		default:
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return nil, nil, err
			}
		}
	}
	if err := expectDelim(dec, '}'); err != nil {
		return nil, nil, err
	}
	for _, k := range []string{"accounts", "services", "transactions"} {
		if !seen[k] {
			errs = append(errs, FieldError{Field: k, Message: "обязательное поле"})
		}
	}
	return s, errs, nil
}

func expectDelim(dec *json.Decoder, want json.Delim) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := tok.(json.Delim); !ok || d != want {
		return fmt.Errorf("ожидалось %q", string(want))
	}
	return nil
}

// streamArray вызывает f для каждого элемента массива; null вместо массива — пустой массив.
func streamArray(dec *json.Decoder, f func(i int) error) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	if tok == nil {
		return nil
	}
	if d, ok := tok.(json.Delim); !ok || d != '[' {
		return errors.New("ожидался массив")
	}
	for i := 0; dec.More(); i++ {
		if err := f(i); err != nil {
			return err
		}
	}
	return expectDelim(dec, ']')
}

// field собирает ошибки «поле обязательно» для одной записи.
type field struct {
	prefix string
	errs   []FieldError
}

func (f *field) missing(name string) {
	f.errs = append(f.errs, FieldError{Field: f.prefix + "." + name, Message: "обязательное поле"})
}

func pick[T any](f *field, name string, vals ...*T) T {
	for _, v := range vals {
		if v != nil {
			return *v
		}
	}
	f.missing(name)
	var zero T
	return zero
}

func alias(v string) string {
	if a, ok := valueAliases[v]; ok {
		return a
	}
	return v
}

func (a rawAccount) convert(i int) (Account, []FieldError) {
	f := &field{prefix: fmt.Sprintf("accounts[%d]", i)}
	out := Account{
		ID:               pick(f, "id", a.ID),
		Email:            pick(f, "email", a.Email),
		Balance:          pick(f, "balance", a.Balance),
		WithdrawBalance:  pick(f, "withdrawBalance", a.WithdrawBalance),
		TotalReplenished: pick(f, "totalReplenished", a.TotalReplenished),
		BonusBalance:     pick(f, "bonusBalance", a.BonusBalance),
		BonusState:       pick(f, "bonusState", a.BonusState),
	}
	return out, f.errs
}

func (v rawService) convert(i int) (Service, []FieldError) {
	f := &field{prefix: fmt.Sprintf("services[%d]", i)}
	out := Service{
		ID:          pick(f, "id", v.ID),
		Name:        pick(f, "name", v.Name),
		OwnerID:     pick(f, "ownerId", v.OwnerID),
		PaymentTerm: alias(pick(f, "paymentTerm", v.PaymentTerm, v.PaymentTem)),
		AutoProlong: pick(f, "autoProlong", v.AutoProlong, v.Autoprolong),
		CreatedAt:   pick(f, "createdAt", v.CreatedAt),
		ExpiresAt:   pick(f, "expiresAt", v.ExpiresAt),
		Status:      alias(pick(f, "status", v.Status)),
		Price:       pick(f, "price", v.Price, v.Pice),
	}
	return out, f.errs
}

func (v rawTransaction) convert(i int) (Transaction, []FieldError) {
	f := &field{prefix: fmt.Sprintf("transactions[%d]", i)}
	out := Transaction{
		ID:          pick(f, "id", v.ID),
		Amount:      pick(f, "amount", v.Amount),
		Status:      alias(pick(f, "status", v.Status)),
		PerformedAt: v.PerformedAt,
		CreatedAt:   pick(f, "createdAt", v.CreatedAt),
		Type:        pick(f, "type", v.Type),
		ServiceID:   pick(f, "serviceId", v.ServiceID),
	}
	return out, f.errs
}

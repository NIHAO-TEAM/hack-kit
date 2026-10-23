// Package snapgen — детерминированный генератор снимков для проверки загрузки чужого файла:
// размер задаётся числом аккаунтов, крайние наборы — флагом Edge. Один seed — один и тот же файл.
package snapgen

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"time"
)

// Крайние наборы. Пустое значение — обычный снимок.
const (
	EdgeNone       = ""
	EdgeEmpty      = "empty"        // три пустых массива
	EdgeDeleted    = "deleted-only" // все услуги удалены
	EdgeNoTx       = "no-tx"        // ни одного платежа
	EdgeZeroPrice  = "zero-price"   // все цены нулевые
	EdgeFutureDate = "future-dates" // даты услуг и платежей позже asOf
	EdgeHourly     = "hourly-only"  // только почасовые услуги
)

// Edges — все допустимые значения флага -edge.
var Edges = []string{EdgeEmpty, EdgeDeleted, EdgeNoTx, EdgeZeroPrice, EdgeFutureDate, EdgeHourly}

// Options — параметры генерации.
type Options struct {
	Accounts int    // число аккаунтов; у каждого от 1 до 3 услуг
	Seed     uint64 // источник случайности
	Edge     string // один из Edge*
}

// AsOf — момент снимка во всех сгенерированных файлах.
var AsOf = time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC)

type account struct {
	ID               int64  `json:"id"`
	Email            string `json:"email"`
	Balance          int64  `json:"balance"`
	WithdrawBalance  int64  `json:"withdrawBalance"`
	TotalReplenished int64  `json:"totalReplenished"`
	BonusBalance     int64  `json:"bonusBalance"`
	BonusState       string `json:"bonusState"`
}

type service struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	OwnerID     int64     `json:"ownerId"`
	PaymentTerm string    `json:"paymentTerm"`
	AutoProlong bool      `json:"autoProlong"`
	CreatedAt   time.Time `json:"createdAt"`
	ExpiresAt   time.Time `json:"expiresAt"`
	Status      string    `json:"status"`
	Price       int64     `json:"price"`
}

type transaction struct {
	ID          int64      `json:"id"`
	Amount      int64      `json:"amount"`
	Status      string     `json:"status"`
	PerformedAt *time.Time `json:"performedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
	Type        string     `json:"type"`
	ServiceID   int64      `json:"serviceId"`
}

type snapshot struct {
	AsOf         time.Time     `json:"asOf"`
	Accounts     []account     `json:"accounts"`
	Services     []service     `json:"services"`
	Transactions []transaction `json:"transactions"`
}

var (
	terms    = []string{"hour", "week", "month", "quarter_year", "half_year", "year"}
	statuses = []string{"active", "active", "active", "blocked", "deleted", "suspended", "prolong_wait"}
)

// Generate возвращает тело PUT /source/snapshot.
func Generate(o Options) ([]byte, error) {
	if o.Accounts < 0 {
		return nil, fmt.Errorf("snapgen: аккаунтов %d", o.Accounts)
	}
	known := o.Edge == EdgeNone
	for _, e := range Edges {
		known = known || e == o.Edge
	}
	if !known {
		return nil, fmt.Errorf("snapgen: неизвестный набор %q", o.Edge)
	}
	rng := rand.New(rand.NewPCG(o.Seed, o.Seed^0x9e3779b97f4a7c15))
	s := snapshot{AsOf: AsOf, Accounts: []account{}, Services: []service{}, Transactions: []transaction{}}
	if o.Edge == EdgeEmpty {
		return json.Marshal(s)
	}
	var svcID, txID int64
	for i := 1; i <= o.Accounts; i++ {
		topUp := int64(rng.IntN(20000))
		s.Accounts = append(s.Accounts, account{
			ID: int64(i), Email: fmt.Sprintf("user%d@gen.test", i),
			Balance: int64(rng.IntN(5000)), TotalReplenished: topUp,
			BonusBalance: int64(rng.IntN(500)), BonusState: []string{"locked", "unlocked"}[rng.IntN(2)],
		})
		for n := 1 + rng.IntN(3); n > 0; n-- {
			svcID++
			v := service{
				ID: svcID, Name: fmt.Sprintf("srv-%d", svcID), OwnerID: int64(i),
				PaymentTerm: terms[rng.IntN(len(terms))], AutoProlong: rng.IntN(2) == 0,
				Status: statuses[rng.IntN(len(statuses))], Price: int64(100 + rng.IntN(5000)),
			}
			age := time.Duration(1+rng.IntN(400)) * 24 * time.Hour
			v.CreatedAt = AsOf.Add(-age)
			v.ExpiresAt = AsOf.Add(time.Duration(rng.IntN(60)-30) * 24 * time.Hour)
			if v.ExpiresAt.Before(v.CreatedAt) {
				v.ExpiresAt = v.CreatedAt
			}
			switch o.Edge {
			case EdgeDeleted:
				v.Status = "deleted"
			case EdgeZeroPrice:
				v.Price = 0
			case EdgeHourly:
				v.PaymentTerm = "hour"
			case EdgeFutureDate:
				v.CreatedAt = AsOf.Add(time.Duration(1+rng.IntN(300)) * 24 * time.Hour)
				v.ExpiresAt = v.CreatedAt.Add(time.Duration(1+rng.IntN(60)) * 24 * time.Hour)
			}
			s.Services = append(s.Services, v)
			if o.Edge == EdgeNoTx {
				continue
			}
			for k := rng.IntN(5); k > 0; k-- {
				txID++
				t := transaction{
					ID: txID, Amount: int64(rng.IntN(1001)), Status: "performed",
					Type: []string{"prolong", "order"}[rng.IntN(2)], ServiceID: v.ID,
				}
				t.CreatedAt = v.CreatedAt.Add(time.Duration(rng.IntN(48)) * time.Hour)
				if rng.IntN(5) == 0 {
					t.Status = "not_performed"
				} else {
					p := t.CreatedAt.Add(time.Duration(rng.IntN(60)) * time.Minute)
					t.PerformedAt = &p
				}
				s.Transactions = append(s.Transactions, t)
			}
		}
	}
	return json.Marshal(s)
}

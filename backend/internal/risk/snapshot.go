package risk

import "time"

// Статусы услуги.
const (
	StatusActive      = "active"
	StatusSuspended   = "suspended"
	StatusProlongWait = "prolong_wait"
	StatusDeleted     = "deleted"
	StatusBlocked     = "blocked"
)

// Account — аккаунт клиента. Деньги — целые центы.
type Account struct {
	ID               int64  `json:"id"`
	Email            string `json:"email"`
	Balance          int64  `json:"balance"`
	WithdrawBalance  int64  `json:"withdrawBalance"`
	TotalReplenished int64  `json:"totalReplenished"`
	BonusBalance     int64  `json:"bonusBalance"`
	BonusState       string `json:"bonusState"`
}

// Service — услуга аккаунта. Price — цена за один период PaymentTerm в центах.
type Service struct {
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

// Transaction — операция по услуге.
type Transaction struct {
	ID          int64      `json:"id"`
	Amount      int64      `json:"amount"`
	Status      string     `json:"status"`
	PerformedAt *time.Time `json:"performedAt,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	Type        string     `json:"type"`
	ServiceID   int64      `json:"serviceId"`
}

// Snapshot — входные данные модели: срез данных организации на момент AsOf.
type Snapshot struct {
	AsOf         time.Time     `json:"asOf"`
	Accounts     []Account     `json:"accounts"`
	Services     []Service     `json:"services"`
	Transactions []Transaction `json:"transactions"`
}

// IsLive — услуга живая: active, suspended или prolong_wait.
func (s Service) IsLive() bool {
	switch s.Status {
	case StatusActive, StatusSuspended, StatusProlongWait:
		return true
	}
	return false
}

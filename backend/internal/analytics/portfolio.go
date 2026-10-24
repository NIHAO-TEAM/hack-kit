package analytics

import (
	"time"

	"example.com/app/internal/platform/api/gen"
)

// Portfolio — клиенты последнего пересчёта в виде, готовом к выдаче. Его собирает сервер
// из результата модели и снимка: модуль analytics ядро модели не знает.
type Portfolio struct {
	Version   int64
	Customers []Customer
}

// Customer — клиент с оценкой. Level — high, medium, low, unscored или lost.
type Customer struct {
	Row      gen.CustomerRow
	Account  gen.Account
	Services []gen.Service
	// Risks — оценка каждой живой услуги; Features — признаки услуги с наибольшим риском.
	Risks    []gen.ServiceRisk
	Features []gen.Feature
	Actions  []gen.Action
	// ActionCombo — эффект совместного применения действий, центы; nil, если действий меньше двух.
	ActionCombo *int64
}

// Feed отдаёт последний портфель; nil, пока ни один расчёт не удался.
type Feed interface {
	Portfolio() *Portfolio
}

// Window — окно спасения: услуга ждёт оплаты, пока не пройдёт срок удаления.
const Window = 5 * 24 * time.Hour

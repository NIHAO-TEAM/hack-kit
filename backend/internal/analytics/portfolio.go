package analytics

import (
	"cmp"
	"slices"
	"sync"
	"time"

	"example.com/app/internal/platform/api/gen"
)

// Portfolio — клиенты последнего пересчёта в виде, готовом к выдаче. Его собирает сервер
// из результата модели и снимка: модуль analytics ядро модели не знает.
type Portfolio struct {
	Version   int64
	Customers []Customer
	// Reasons — идентификаторы всех причин (признаков модели) для фильтра `reason`; клиентов по ним может не быть.
	Reasons []string

	// Модель чтения (readmodel.go): строится один раз, после этого не меняется.
	once sync.Once
	rm   *readModel
}

// PatchPortfolio строит портфель версии version из prev, заменяя клиентов changed (по Row.Id).
// Набор клиентов тот же, что в prev; клиента, которого в prev нет, заменить нельзя — тогда nil, и
// портфель собирают заново. Модель чтения строится из модели prev, а не с нуля (см. readModel.patched);
// prev не меняется.
func PatchPortfolio(prev *Portfolio, version int64, changed []Customer) *Portfolio {
	pf := &Portfolio{Version: version, Customers: slices.Clone(prev.Customers)}
	pos := make([]int32, 0, len(changed))
	for _, c := range changed {
		i, ok := slices.BinarySearchFunc(prev.Customers, c.Row.Id, func(x Customer, id int64) int { return cmp.Compare(x.Row.Id, id) })
		if !ok {
			return nil
		}
		pf.Customers[i] = c
		pos = append(pos, int32(i))
	}
	slices.Sort(pos)
	pos = slices.Compact(pos)
	rm := prev.index().patched(pf.Customers, pos)
	pf.once.Do(func() { pf.rm = rm })
	return pf
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
	// Loss — ожидаемая потеря по причинам (признак → центы), только ненулевые; сумма равна потере по живым услугам.
	Loss map[string]int64
}

// Feed отдаёт последний портфель; nil, пока ни один расчёт не удался.
type Feed interface {
	Portfolio() *Portfolio
}

// Window — окно спасения: услуга ждёт оплаты, пока не пройдёт срок удаления.
const Window = 5 * 24 * time.Hour

package analytics

import (
	"cmp"
	"encoding/json"
	"slices"
	"strings"
	"sync"

	"example.com/app/internal/platform/api/gen"
)

// Модель чтения: порядки клиентов под каждую сортировку списка строятся один раз на портфель.
// Портфель после сборки не меняется (новый пересчёт даёт новый), поэтому читатели берут порядки
// без блокировок и не сортируют на запрос. Порядок — срез позиций в Portfolio.Customers (по 4 байта
// на клиента), строки не копируются.

var sortKeys = []gen.ListCustomersParamsSort{"id", "email", "balance", "totalReplenished", "score", "expectedLoss"}

type readModel struct {
	// orders[ключ][0] — по возрастанию, [1] — по убыванию; при равных значениях всегда по id.
	orders [][2][]int32
	// raw[i] — JSON строки списка клиента i без поля task (оно зависит от запроса); собирается один раз.
	raw [][]byte
	// byReason[признак][0] — позиции клиентов с ненулевой потерей по причине по возрастанию этой потери,
	// [1] — по убыванию; при равных значениях по id.
	byReason map[string][2][]int32
}

// BuildIndex строит порядки сразу; без вызова они строятся при первом чтении списка.
// Вызывать безопасно из нескольких горутин: строится один раз.
func (pf *Portfolio) BuildIndex() { pf.index() }

func (pf *Portfolio) index() *readModel {
	pf.once.Do(func() { pf.rm = buildReadModel(pf.Customers) })
	return pf.rm
}

func buildReadModel(cs []Customer) *readModel {
	lower := make([]string, len(cs))
	for i := range cs {
		lower[i] = strings.ToLower(cs[i].Row.Email)
	}
	rm := &readModel{orders: make([][2][]int32, len(sortKeys))}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		rm.raw = make([][]byte, len(cs))
		for i := range cs {
			row := cs[i].Row
			row.Task = nil
			rm.raw[i], _ = json.Marshal(row)
		}
	}()
	for k, key := range sortKeys {
		for d := 0; d < 2; d++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				rm.orders[k][d] = sortedPositions(cs, lower, key, d == 1)
			}()
		}
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		rm.byReason = reasonOrders(cs)
	}()
	wg.Wait()
	return rm
}

// reasonOrders строит для каждой причины список клиентов с ненулевой потерей по ней.
func reasonOrders(cs []Customer) map[string][2][]int32 {
	byReason := map[string][2][]int32{}
	for i := range cs {
		for id := range cs[i].Loss {
			o := byReason[id]
			o[1] = append(o[1], int32(i))
			byReason[id] = o
		}
	}
	for id, o := range byReason {
		desc := o[1]
		slices.SortFunc(desc, func(i, j int32) int {
			if c := cmp.Compare(cs[j].Loss[id], cs[i].Loss[id]); c != 0 {
				return c
			}
			return cmp.Compare(cs[i].Row.Id, cs[j].Row.Id)
		})
		asc := slices.Clone(desc)
		slices.SortFunc(asc, func(i, j int32) int {
			if c := cmp.Compare(cs[i].Loss[id], cs[j].Loss[id]); c != 0 {
				return c
			}
			return cmp.Compare(cs[i].Row.Id, cs[j].Row.Id)
		})
		o[0] = asc
		byReason[id] = o
	}
	return byReason
}

// reasonOrder — клиенты причины в порядке её потери; нет клиентов — пустой срез.
func (rm *readModel) reasonOrder(reason string, desc bool) []int32 {
	o := rm.byReason[reason]
	if desc {
		return o[1]
	}
	return o[0]
}

// sortedPositions повторяет прежнюю сортировку списка: по полю, при равных — по id; по скору
// и потере «нет оценки» и «потерян» всегда внизу, по id.
func sortedPositions(cs []Customer, lower []string, key gen.ListCustomersParamsSort, desc bool) []int32 {
	pos := make([]int32, len(cs))
	for i := range pos {
		pos[i] = int32(i)
	}
	pin := key == "score" || key == "expectedLoss"
	cmpPos := func(i, j int32) int {
		x, y := &cs[i].Row, &cs[j].Row
		switch key {
		case "id":
			return cmp.Compare(x.Id, y.Id)
		case "email":
			return cmp.Compare(lower[i], lower[j])
		case "balance":
			return cmp.Compare(x.Balance, y.Balance)
		case "totalReplenished":
			return cmp.Compare(x.TotalReplenished, y.TotalReplenished)
		case "score":
			return cmp.Compare(deref(x.Score), deref(y.Score))
		}
		return cmp.Compare(x.ExpectedLossCents, y.ExpectedLossCents)
	}
	slices.SortFunc(pos, func(i, j int32) int {
		x, y := &cs[i].Row, &cs[j].Row
		if pin {
			xs, ys := isScored(*x), isScored(*y)
			switch {
			case xs != ys:
				if xs {
					return -1
				}
				return 1
			case !xs:
				return cmp.Compare(x.Id, y.Id)
			}
		}
		c := cmpPos(i, j)
		if desc {
			c = -c
		}
		if c != 0 {
			return c
		}
		return cmp.Compare(x.Id, y.Id)
	})
	return pos
}

func (rm *readModel) order(key gen.ListCustomersParamsSort, desc bool) ([]int32, bool) {
	k := slices.Index(sortKeys, key)
	if k < 0 {
		return nil, false
	}
	d := 0
	if desc {
		d = 1
	}
	return rm.orders[k][d], true
}

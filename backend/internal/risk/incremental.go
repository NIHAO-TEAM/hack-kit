package risk

import (
	"cmp"
	"context"
	"errors"
	"maps"
	"slices"
	"sort"
)

// ErrNeedFull — частичный пересчёт невозможен или небезопасен: нужен полный (клиента нет в прежнем
// результате, снимок сменил asOf, у проверки пропали перечисленные услуги). Это не сбой.
var ErrNeedFull = errors.New("risk: нужен полный пересчёт")

// Rescore пересчитывает только клиентов ids и возвращает результат, равный Score над снимком, где
// строки этих клиентов заменены. prev — результат по прежнему снимку; sub — актуальные строки
// клиентов ids (счета, их услуги и платежи), AsOf снимка совпадает с prev.AsOf.
//
// Клиенты друг от друга не зависят (признаки, проверки и действия видят только данные клиента),
// поэтому сводку и проверки качества достаточно поправить разностью: вычесть старый вклад клиента и
// прибавить новый. prev не меняется.
func Rescore(ctx context.Context, prev Result, sub Snapshot, ids []int64, st Settings, opt Options) (Result, error) {
	fs, cs, as := features.all(), checks.all(), actions.all()
	if !sub.AsOf.Equal(prev.AsOf) || len(prev.Checks) != len(cs) {
		return Result{}, ErrNeedFull
	}
	for i, c := range cs {
		if prev.Checks[i].ID != c.ID() {
			return Result{}, ErrNeedFull
		}
	}
	ids = slices.Clone(ids)
	slices.Sort(ids)
	ids = slices.Compact(ids)

	pos := make([]int, len(ids)) // место клиента в prev.Customers
	for k, id := range ids {
		i, ok := slices.BinarySearchFunc(prev.Customers, id, func(c Customer, id int64) int { return cmp.Compare(c.ID, id) })
		if !ok {
			return Result{}, ErrNeedFull
		}
		pos[k] = i
	}
	accounts := make(map[int64]Account, len(ids))
	for _, a := range sub.Accounts {
		accounts[a.ID] = a
	}
	svcByOwner := groupBy(sub.Services, func(sv Service) int64 { return sv.OwnerID })
	txByService := groupBy(sub.Transactions, func(t Transaction) int64 { return t.ServiceID })
	for _, id := range ids {
		if _, ok := accounts[id]; !ok {
			return Result{}, ErrNeedFull
		}
	}

	fresh := make([]Customer, len(ids))
	err := parallelFor(ctx, len(ids), opt, func(w *scratch, k int) {
		a := accounts[ids[k]]
		fresh[k] = scoreAccount(w, sub.AsOf, a, svcByOwner[a.ID], txByService, st, prev.AsOf, fs, cs, as)
	})
	if err != nil {
		return Result{}, err
	}

	res := Result{AsOf: prev.AsOf, Customers: slices.Clone(prev.Customers)}
	res.Summary = prev.Summary
	res.Summary.Levels = maps.Clone(prev.Summary.Levels)
	res.Summary.QualityCounts = maps.Clone(prev.Summary.QualityCounts)
	sums := make(map[string]*FeatureLoss, len(prev.Summary.LossByFeature))
	for _, fl := range prev.Summary.LossByFeature {
		sums[fl.ID] = &FeatureLoss{ID: fl.ID, LossCents: fl.LossCents, Customers: fl.Customers}
	}
	touched := make(map[int64]struct{}, len(ids))
	for k, i := range pos {
		old := prev.Customers[i]
		contribute(&res.Summary, old, -1)
		contribute(&res.Summary, fresh[k], 1)
		addFeatureLoss(sums, old, -1)
		addFeatureLoss(sums, fresh[k], 1)
		res.Customers[i] = fresh[k]
		touched[ids[k]] = struct{}{}
	}
	res.Summary.LossByFeature = sortedFeatureLoss(sums)

	checks, err := rescoreChecks(prev.Checks, cs, prev.Customers, pos, fresh, ids, touched)
	if err != nil {
		return Result{}, err
	}
	res.Checks = checks
	return res, nil
}

// rescoreChecks поправляет проверки качества: число случаев — разностью, список первых услуг —
// без затронутых клиентов плюс их новые случаи. Список в полном счёте — первые
// maxCheckItems случаев по возрастанию клиента; если после правки в нём недостаёт записей,
// которых раньше не было видно, точный список можно получить только полным счётом.
func rescoreChecks(prev []CheckResult, cs []Check, old []Customer, pos []int, fresh []Customer, ids []int64, touched map[int64]struct{}) ([]CheckResult, error) {
	type hit struct {
		checkID string
		item    CheckItem
	}
	var removed, added []hit
	for k, i := range pos {
		eachCheckHit(old[i], ids[k], func(id string, item CheckItem) { removed = append(removed, hit{id, item}) })
		eachCheckHit(fresh[k], ids[k], func(id string, item CheckItem) { added = append(added, hit{id, item}) })
	}
	out := make([]CheckResult, len(prev))
	for j, c := range cs {
		r := prev[j]
		truncated := len(r.Items) > 0 && r.Count > len(r.Items) // список прежнего расчёта обрезан
		for _, h := range removed {
			if h.checkID == r.ID {
				r.Count--
			}
		}
		var items []CheckItem
		for _, it := range r.Items {
			if _, gone := touched[it.AccountID]; !gone {
				items = append(items, it)
			}
		}
		// Прежний список обрезан (случаев больше, чем в нём): дальше последнего клиента списка он
		// не знает, а новые случаи оттуда могли бы встать впереди невидимых. Их не берём.
		var horizon int64
		if truncated {
			horizon = r.Items[len(r.Items)-1].AccountID
		}
		for _, h := range added {
			if h.checkID == r.ID {
				r.Count++
				if !truncated || h.item.AccountID <= horizon {
					items = append(items, h.item)
				}
			}
		}
		// клиентов в списке по порядку; внутри клиента порядок обхода сохраняется (сортировка устойчива)
		sort.SliceStable(items, func(a, b int) bool { return items[a].AccountID < items[b].AccountID })
		if len(items) > maxCheckItems {
			items = items[:maxCheckItems]
		}
		if len(items) != min(r.Count, maxCheckItems) {
			return nil, ErrNeedFull
		}
		r.Items, r.OK = items, r.Count == 0
		r.Message = checkMessage(c, r.Count)
		out[j] = r
	}
	return out, nil
}

package app

import (
	"bytes"
	"context"
	"strconv"
	"testing"

	"example.com/app/internal/analytics"
	"example.com/app/internal/risk"
	"example.com/app/internal/source"
	"example.com/app/tools/snapgen"
)

func benchSnapshot(tb testing.TB, accounts int) risk.Snapshot {
	tb.Helper()
	body, err := snapgen.Generate(snapgen.Options{Accounts: accounts, Seed: 11})
	if err != nil {
		tb.Fatal(err)
	}
	snap, _, err := source.ParseSnapshot(bytes.NewReader(body))
	if err != nil || snap == nil {
		tb.Fatalf("разбор синтетического снимка: %v", err)
	}
	return ToRiskSnapshot(*snap)
}

// BenchmarkOneEventFull100k — цена одного события до частичного пересчёта: полный счёт модели
// и сборка портфеля с моделью чтения на 100 000 клиентов.
func BenchmarkOneEventFull100k(b *testing.B) {
	rs := benchSnapshot(b, 100_000)
	st := risk.Defaults()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := risk.Score(rs, st, rs.AsOf)
		buildPortfolio(res, rs, int64(i)).BuildIndex()
	}
}

// BenchmarkOneEventPartial100k — то же событие после правки: пересчёт k клиентов поверх прежнего
// результата, сводка разностью и портфель с моделью чтения из прежней. Чтение строк из базы не входит.
func BenchmarkOneEventPartial100k(b *testing.B) {
	rs := benchSnapshot(b, 100_000)
	st := risk.Defaults()
	prev := risk.Score(rs, st, rs.AsOf)
	pf := buildPortfolio(prev, rs, 1)
	pf.BuildIndex()

	svcs := map[int64][]risk.Service{}
	for _, sv := range rs.Services {
		svcs[sv.OwnerID] = append(svcs[sv.OwnerID], sv)
	}
	txs := map[int64][]risk.Transaction{}
	for _, t := range rs.Transactions {
		txs[t.ServiceID] = append(txs[t.ServiceID], t)
	}
	for _, k := range []int{1, 10, 100, 1000} {
		b.Run(strconv.Itoa(k), func(b *testing.B) {
			b.ReportAllocs()
			res, cur := prev, pf
			for i := 0; i < b.N; i++ {
				sub := risk.Snapshot{AsOf: rs.AsOf}
				ids := make([]int64, k)
				for j := range ids {
					a := rs.Accounts[(i*k+j)*7919%len(rs.Accounts)]
					a.Balance += 100
					ids[j] = a.ID
					sub.Accounts = append(sub.Accounts, a)
					for _, sv := range svcs[a.ID] {
						sub.Services = append(sub.Services, sv)
						sub.Transactions = append(sub.Transactions, txs[sv.ID]...)
					}
				}
				var err error
				if res, err = risk.Rescore(context.Background(), res, sub, ids, st, risk.Options{}); err != nil {
					b.Fatal(err)
				}
				if cur = analytics.PatchPortfolio(cur, int64(i+2), changedCustomers(res, sub)); cur == nil {
					b.Fatal("портфель не собран")
				}
			}
		})
	}
}

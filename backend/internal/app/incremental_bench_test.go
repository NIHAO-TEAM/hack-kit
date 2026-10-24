package app

import (
	"bytes"
	"testing"

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

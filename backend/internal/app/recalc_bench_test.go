package app_test

import (
	"bytes"
	"testing"

	"example.com/app/internal/app"
	"example.com/app/internal/risk"
	"example.com/app/internal/source"
	"example.com/app/tools/snapgen"
)

func syntheticSnapshot(tb testing.TB, accounts int) risk.Snapshot {
	tb.Helper()
	body, err := snapgen.Generate(snapgen.Options{Accounts: accounts, Seed: 11})
	if err != nil {
		tb.Fatal(err)
	}
	snap, _, err := source.ParseSnapshot(bytes.NewReader(body))
	if err != nil || snap == nil {
		tb.Fatalf("разбор синтетического снимка: %v", err)
	}
	return app.ToRiskSnapshot(*snap)
}

func benchRecalc(b *testing.B, accounts int) {
	rs := syntheticSnapshot(b, accounts)
	st := risk.Defaults()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		risk.Score(rs, st, rs.AsOf)
	}
}

func BenchmarkRecalc10k(b *testing.B)  { benchRecalc(b, 10_000) }
func BenchmarkRecalc100k(b *testing.B) { benchRecalc(b, 100_000) }

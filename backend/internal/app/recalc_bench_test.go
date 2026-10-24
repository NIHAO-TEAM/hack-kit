package app_test

import (
	"bytes"
	"context"
	"strconv"
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

func BenchmarkRecalc100kBatch(b *testing.B) {
	rs := syntheticSnapshot(b, 100_000)
	st := risk.Defaults()
	for _, batch := range []int{64, 256, 512, 1024} {
		b.Run(strconv.Itoa(batch), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := risk.ScoreContext(context.Background(), rs, st, rs.AsOf, risk.Options{BatchSize: batch}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

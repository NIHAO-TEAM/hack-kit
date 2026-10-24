package analytics_test

import (
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"example.com/app/internal/analytics"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
)

// bigPortfolio — n клиентов с разбросом уровня, скора и потери; порядок по id.
func bigPortfolio(n int) *analytics.Portfolio {
	rng := rand.New(rand.NewSource(11))
	levels := []gen.RiskLevel{gen.High, gen.Medium, gen.Low, gen.Low, gen.Unscored, gen.Lost}
	cs := make([]analytics.Customer, n)
	for i := range cs {
		id := int64(i + 1)
		lvl := levels[rng.Intn(len(levels))]
		c := gen.CustomerRow{
			Id: id, Email: fmt.Sprintf("user%d@example.test", rng.Intn(n*10)), Level: lvl,
			Balance: int64(rng.Intn(100000)), TotalReplenished: int64(rng.Intn(1000000)),
		}
		if lvl != gen.Unscored && lvl != gen.Lost {
			s := rng.Float64() * 100
			c.Score = &s
			c.ExpectedLossCents = int64(rng.Intn(500000))
		}
		cs[i] = analytics.Customer{Row: c}
	}
	return &analytics.Portfolio{Version: 1, Customers: cs}
}

func benchList(b *testing.B, p gen.ListCustomersParams) {
	pf := bigPortfolio(100_000)
	api := analytics.New(nil, nil, clock.NewFake(asOf)).WithFeed(fakeFeed{pf})
	pf.BuildIndex() // сборка модели чтения — отдельный замер BenchmarkBuildReadModel100k
	req := httptest.NewRequest(http.MethodGet, "/api/v1/customers", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		api.ListCustomers(rec, req, p)
		if rec.Code != 200 {
			b.Fatalf("статус %d: %s", rec.Code, rec.Body)
		}
	}
}

func BenchmarkListByLoss100k(b *testing.B) { benchList(b, gen.ListCustomersParams{}) }

func BenchmarkListByEmail100k(b *testing.B) {
	s := gen.ListCustomersParamsSort("email")
	benchList(b, gen.ListCustomersParams{Sort: &s})
}

func BenchmarkBuildReadModel100k(b *testing.B) {
	cs := bigPortfolio(100_000).Customers
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pf := &analytics.Portfolio{Version: 1, Customers: cs}
		pf.BuildIndex()
	}
}

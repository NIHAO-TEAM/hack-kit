package retention

import (
	"reflect"
	"strconv"
	"testing"
	"time"

	"example.com/app/internal/analytics"
	"example.com/app/internal/platform/api/gen"
)

func TestObserve(t *testing.T) {
	exp := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	score := func(v float64) *float64 { return &v }
	base := gen.TaskBaseline{
		Level: gen.High, Score: score(86.95), ExpectedLossCents: 50000, Balance: 1000,
		Services: []gen.TaskServiceBaseline{{Id: 7, Status: gen.ProlongWait, ExpiresAt: exp}},
	}
	cust := func(mod func(c *analytics.Customer)) analytics.Customer {
		c := analytics.Customer{
			Row:      gen.CustomerRow{Level: gen.High, Score: score(86.95), ExpectedLossCents: 50000, Balance: 1000},
			Services: []gen.Service{{Id: 7, Name: "VPS-1", Status: gen.ProlongWait, ExpiresAt: exp}},
		}
		mod(&c)
		return c
	}
	for name, c := range map[string]struct {
		seen int64
		cur  analytics.Customer
		want []Observation
	}{
		"без изменений": {1000, cust(func(*analytics.Customer) {}), nil},
		"пополнение": {1000, cust(func(c *analytics.Customer) { c.Row.Balance = 61000 }),
			[]Observation{{"balance:61000", "Баланс пополнен на 600,00 €"}}},
		"пополнение уже записано": {61000, cust(func(c *analytics.Customer) { c.Row.Balance = 61000 }), nil},
		"баланс упал":             {1000, cust(func(c *analytics.Customer) { c.Row.Balance = 10 }), nil},
		"автопродление": {1000, cust(func(c *analytics.Customer) { c.Services[0].AutoProlong = true }),
			[]Observation{{"autoprolong:7", "Включено автопродление услуги VPS-1"}}},
		"продление": {1000, cust(func(c *analytics.Customer) { c.Services[0].ExpiresAt = exp.AddDate(0, 1, 0) }),
			[]Observation{{"renewed:7:" + strconv.FormatInt(exp.AddDate(0, 1, 0).Unix(), 10), "Услуга VPS-1 продлена до 01.12.2026"}}},
		"возврат в active": {1000, cust(func(c *analytics.Customer) { c.Services[0].Status = gen.Active }),
			[]Observation{{"renewed:7:" + strconv.FormatInt(exp.Unix(), 10), "Услуга VPS-1 продлена до 01.11.2026"}}},
		"заблокирована": {1000, cust(func(c *analytics.Customer) { c.Services[0].Status = gen.Blocked }),
			[]Observation{{"status:7:blocked", "Услуга VPS-1 заблокирована"}}},
		"удалена": {1000, cust(func(c *analytics.Customer) { c.Services[0].Status = gen.Deleted }),
			[]Observation{{"status:7:deleted", "Услуга VPS-1 удалена"}}},
		"смена уровня": {1000, cust(func(c *analytics.Customer) {
			c.Row.Level, c.Row.Score, c.Row.ExpectedLossCents = gen.Low, score(40), 10000
		}), []Observation{{"level:low:40,00", "Скор 86,95 → 40,00, ожидаемая потеря −400,00 €"}}},
		"скор без смены уровня": {1000, cust(func(c *analytics.Customer) { c.Row.Score = score(70) }), nil},
	} {
		t.Run(name, func(t *testing.T) {
			got := observe(base, c.seen, c.cur)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("observe = %+v, ждали %+v", got, c.want)
			}
		})
	}
}

// BenchmarkObserve100 — сравнение для 100 открытых задач: добавка ко времени одного пересчёта.
func BenchmarkObserve100(b *testing.B) {
	exp := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	base := gen.TaskBaseline{Level: gen.High, Balance: 1000, Services: []gen.TaskServiceBaseline{
		{Id: 1, Status: gen.ProlongWait, ExpiresAt: exp}, {Id: 2, Status: gen.Active, ExpiresAt: exp},
	}}
	c := analytics.Customer{
		Row: gen.CustomerRow{Level: gen.Low, Balance: 61000},
		Services: []gen.Service{
			{Id: 1, Name: "VPS-1", Status: gen.Active, AutoProlong: true, ExpiresAt: exp.AddDate(0, 1, 0)},
			{Id: 2, Name: "VPS-2", Status: gen.Blocked, ExpiresAt: exp},
		},
	}
	b.ResetTimer()
	for range b.N {
		for range 100 {
			_ = observe(base, 1000, c)
		}
	}
}

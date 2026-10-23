package risk

import (
	"math"
	"time"
)

var periodMonths = map[string]int{"month": 1, "quarter_year": 3, "half_year": 6, "year": 12}

// addTerm прибавляет n периодов term к v. Месяцы календарные, день не больше последнего дня месяца.
func addTerm(v time.Time, term string, n int) time.Time {
	switch term {
	case "hour":
		return v.Add(time.Duration(n) * time.Hour)
	case "week":
		return v.AddDate(0, 0, 7*n)
	}
	months := periodMonths[term] * n
	y, m, d := v.Date()
	first := time.Date(y, m, 1, 0, 0, 0, 0, time.UTC).AddDate(0, months, 0)
	last := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if d > last {
		d = last
	}
	return time.Date(first.Year(), first.Month(), d, v.Hour(), v.Minute(), v.Second(), v.Nanosecond(), v.Location())
}

// charges — число списаний услуги в окне [asOf, asOf + hours) по правилу 3.
func charges(s Service, asOf time.Time, hours int) int {
	if s.Status == StatusProlongWait {
		return 1
	}
	if s.PaymentTerm == "hour" {
		return hours
	}
	if _, ok := periodMonths[s.PaymentTerm]; !ok && s.PaymentTerm != "week" {
		return 0
	}
	end := asOf.Add(time.Duration(hours) * time.Hour)
	base := s.ExpiresAt
	if base.Before(asOf) {
		base = asOf
	}
	n := 0
	for addTerm(base, s.PaymentTerm, n).Before(end) {
		n++
	}
	return n
}

// amount — деньги по услуге за окно в центах (правило 4): price × списания;
// при per_month у hour-услуги price берётся за 720 часов.
func amount(s Service, asOf time.Time, hours int, basis string) int64 {
	if s.PaymentTerm == "hour" && basis == PricePerMonth && s.Status != StatusProlongWait {
		return int64(math.RoundToEven(float64(s.Price) * float64(hours) / monthHours))
	}
	return s.Price * int64(charges(s, asOf, hours))
}

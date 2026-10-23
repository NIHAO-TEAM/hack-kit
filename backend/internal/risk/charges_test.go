package risk

import (
	"testing"
	"time"
)

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestCharges(t *testing.T) {
	asOf := ts("2026-10-19T15:00:00Z")
	cases := []struct {
		name string
		s    Service
		h    int
		want int
	}{
		{"hour 720", Service{PaymentTerm: "hour", Status: StatusActive, ExpiresAt: asOf}, 720, 720},
		{"hour 168", Service{PaymentTerm: "hour", Status: StatusActive, ExpiresAt: asOf}, 168, 168},
		{"prolong_wait", Service{PaymentTerm: "month", Status: StatusProlongWait, ExpiresAt: asOf}, 720, 1},
		{"месяц, просрочена", Service{PaymentTerm: "month", Status: StatusSuspended, ExpiresAt: ts("2026-09-09T15:00:00Z")}, 720, 1},
		{"месяц, просрочена, 168 ч", Service{PaymentTerm: "month", Status: StatusSuspended, ExpiresAt: ts("2026-09-09T15:00:00Z")}, 168, 1},
		{"месяц, срок позже окна", Service{PaymentTerm: "month", Status: StatusActive, ExpiresAt: ts("2026-11-19T14:30:00Z")}, 720, 0},
		{"неделя", Service{PaymentTerm: "week", Status: StatusActive, ExpiresAt: asOf.Add(time.Hour)}, 720, 5},
		{"год", Service{PaymentTerm: "year", Status: StatusActive, ExpiresAt: asOf.Add(24 * time.Hour)}, 720, 1},
		{"неизвестный период", Service{PaymentTerm: "x", Status: StatusActive, ExpiresAt: asOf}, 720, 0},
	}
	for _, c := range cases {
		if got := charges(c.s, asOf, c.h); got != c.want {
			t.Errorf("%s: %d, ждали %d", c.name, got, c.want)
		}
	}
}

func TestAddTermMonthEnd(t *testing.T) {
	base := ts("2027-01-31T10:00:00Z")
	if got := addTerm(base, "month", 1); !got.Equal(ts("2027-02-28T10:00:00Z")) {
		t.Errorf("28.02: %v", got)
	}
	if got := addTerm(base, "month", 2); !got.Equal(ts("2027-03-31T10:00:00Z")) {
		t.Errorf("31.03: %v", got)
	}
	if got := addTerm(ts("2028-01-31T10:00:00Z"), "month", 1); !got.Equal(ts("2028-02-29T10:00:00Z")) {
		t.Errorf("29.02: %v", got)
	}
}

func TestAmount(t *testing.T) {
	asOf := ts("2026-10-19T15:00:00Z")
	hour := Service{PaymentTerm: "hour", Status: StatusActive, Price: 95, ExpiresAt: asOf}
	if got := amount(hour, asOf, 720, PricePerPeriod); got != 68400 {
		t.Errorf("per_period: %d", got)
	}
	if got := amount(hour, asOf, 720, PricePerMonth); got != 95 {
		t.Errorf("per_month R: %d", got)
	}
	if got := amount(hour, asOf, 168, PricePerMonth); got != 22 { // round(95·168/720) = 22,17
		t.Errorf("per_month запас: %d", got)
	}
}

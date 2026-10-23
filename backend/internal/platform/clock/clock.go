// Package clock — единственный источник времени для доменного кода.
package clock

import (
	"sync"
	"time"
)

// Clock отдаёт текущее время.
type Clock interface {
	Now() time.Time
}

// System — настоящие часы.
type System struct{}

// Now возвращает текущее время в UTC.
func (System) Now() time.Time { return time.Now().UTC() }

// Fake — часы для тестов: стоят на месте, пока их не сдвинут.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake создаёт часы, стоящие на t.
func NewFake(t time.Time) *Fake { return &Fake{now: t} }

// Now возвращает заданное время.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance сдвигает часы вперёд на d.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}

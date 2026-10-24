// Package events — шина событий внутри процесса. Её создаёт cmd/api и передаёт модулям.
package events

import (
	"sync"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

// Event — событие шины.
type Event struct {
	Topic   string
	Payload any
}

// DefaultBuffer — размер буфера подписчика у Subscribe.
const DefaultBuffer = 16

// sub — один подписчик. mu защищает канал от записи после закрытия: издатель берёт RLock,
// отписка — Lock. Блокировка на подписчика, а не на шину, поэтому издатели не ждут друг друга.
type sub struct {
	mu     sync.RWMutex
	ch     chan Event
	closed bool
}

// topic — подписчики темы. list неизменяем: запись создаёт новую копию, Publish читает без блокировки.
type topic struct {
	list    atomic.Pointer[[]*sub]
	dropped atomic.Uint64
}

// Bus рассылает события подписчикам темы. Медленный подписчик не блокирует издателя:
// у каждого буфер, при переполнении самое старое событие теряется (и считается).
type Bus struct {
	mu     sync.Mutex // только запись: Subscribe и cancel
	topics sync.Map   // string -> *topic
}

// New создаёт пустую шину.
func New() *Bus { return &Bus{} }

func (b *Bus) topic(name string) *topic {
	if t, ok := b.topics.Load(name); ok {
		return t.(*topic)
	}
	t, _ := b.topics.LoadOrStore(name, &topic{})
	return t.(*topic)
}

// Publish отправляет событие всем подписчикам темы.
func (b *Bus) Publish(name string, payload any) {
	v, ok := b.topics.Load(name)
	if !ok {
		return
	}
	t := v.(*topic)
	list := t.list.Load()
	if list == nil {
		return
	}
	ev := Event{Topic: name, Payload: payload}
	for _, s := range *list {
		s.send(ev, &t.dropped)
	}
}

func (s *sub) send(ev Event, dropped *atomic.Uint64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return
	}
	select {
	case s.ch <- ev:
		return
	default:
	}
	select { // буфер полон: выбрасываем самое старое
	case <-s.ch:
		dropped.Add(1)
	default:
	}
	select {
	case s.ch <- ev:
	default:
		dropped.Add(1) // параллельный издатель успел занять место: теряется уже это событие
	}
}

// Subscribe подписывает на тему с буфером DefaultBuffer. cancel отписывает и закрывает канал.
func (b *Bus) Subscribe(name string) (events <-chan Event, cancel func()) {
	return b.SubscribeBuf(name, DefaultBuffer)
}

// SubscribeBuf — Subscribe с заданным размером буфера (меньше 1 — DefaultBuffer).
func (b *Bus) SubscribeBuf(name string, size int) (events <-chan Event, cancel func()) {
	if size < 1 {
		size = DefaultBuffer
	}
	s := &sub{ch: make(chan Event, size)}
	t := b.topic(name)

	b.mu.Lock()
	t.list.Store(appendSub(t.list.Load(), s))
	b.mu.Unlock()

	var once sync.Once
	return s.ch, func() {
		once.Do(func() {
			b.mu.Lock()
			t.list.Store(removeSub(t.list.Load(), s))
			b.mu.Unlock()
			// Издатель, взявший старую копию списка, ещё может держать RLock: ждём его и закрываем.
			s.mu.Lock()
			s.closed = true
			close(s.ch)
			s.mu.Unlock()
		})
	}
}

func appendSub(old *[]*sub, s *sub) *[]*sub {
	var n []*sub
	if old != nil {
		n = make([]*sub, len(*old), len(*old)+1)
		copy(n, *old)
	}
	n = append(n, s)
	return &n
}

func removeSub(old *[]*sub, s *sub) *[]*sub {
	n := make([]*sub, 0, len(*old))
	for _, x := range *old {
		if x != s {
			n = append(n, x)
		}
	}
	return &n
}

var (
	descDropped = prometheus.NewDesc("churn_events_dropped_total",
		"События, потерянные из-за переполнения буфера подписчика (штуки); topic — тема.", []string{"topic"}, nil)
	descSubscribers = prometheus.NewDesc("churn_events_subscribers",
		"Текущее число подписчиков темы (штуки); topic — тема.", []string{"topic"}, nil)
)

// Collector возвращает сборщик метрик шины для /metrics.
func (b *Bus) Collector() prometheus.Collector { return busCollector{b} }

type busCollector struct{ b *Bus }

func (busCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- descDropped
	ch <- descSubscribers
}

func (c busCollector) Collect(ch chan<- prometheus.Metric) {
	c.b.topics.Range(func(k, v any) bool {
		name, t := k.(string), v.(*topic)
		n := 0
		if l := t.list.Load(); l != nil {
			n = len(*l)
		}
		ch <- prometheus.MustNewConstMetric(descDropped, prometheus.CounterValue, float64(t.dropped.Load()), name)
		ch <- prometheus.MustNewConstMetric(descSubscribers, prometheus.GaugeValue, float64(n), name)
		return true
	})
}

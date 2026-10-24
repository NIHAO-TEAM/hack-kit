package source

import (
	"math/rand"
	"sort"
	"time"
)

// Виды событий потока.
const (
	EventTopUp          = "top_up"
	EventAutoprolongOn  = "autoprolong_on"
	EventAutoprolongOff = "autoprolong_off"
	EventServiceStop    = "service_stop"
	EventServiceResume  = "service_resume"
	EventRenewalFailed  = "renewal_failed"
	EventProlongWait    = "prolong_wait"
)

// StreamEvent — одно событие потока. Значения абсолютные, как в PATCH: событие применяется
// patchService и patchAccount, а не отдельным кодом записи.
type StreamEvent struct {
	Kind      string
	ServiceID int64
	AccountID int64
	// Balance — новый баланс счёта AccountID; у событий без правки счёта nil.
	Balance *int64
	// Status, AutoProlong, ExpiresAt — правка услуги ServiceID; nil — поле не меняется.
	Status      *string
	AutoProlong *bool
	ExpiresAt   *time.Time
}

// Доли смеси в процентах: пополнение, автопродление, остановка и возобновление, неудачное
// продление, переход в ожидание продления.
const (
	shareTopUp      = 40
	shareAutoprolog = 25
	shareStop       = 15
	shareFailed     = 10
	// остаток до 100 — переход в ожидание продления
)

// GenerateStream строит n событий над снимком по зерну. Чистая функция: часов и ввода-вывода нет,
// то же зерно на том же снимке даёт ту же последовательность. Генератор ведёт собственную копию
// состояния услуг и счетов, поэтому каждое событие применимо к снимку после предыдущих.
// Пустой снимок без счетов событий не даёт.
func GenerateStream(s Snapshot, n int, seed int64) []StreamEvent {
	if len(s.Accounts) == 0 || n <= 0 {
		return nil
	}
	p := &planner{rng: rand.New(rand.NewSource(seed))} //nolint:gosec // воспроизводимость, не безопасность
	if s.AsOf != nil {
		p.asOf = s.AsOf.UTC()
	}
	p.accounts = append([]Account(nil), s.Accounts...)
	sort.Slice(p.accounts, func(i, j int) bool { return p.accounts[i].ID < p.accounts[j].ID })
	p.balance = make(map[int64]int64, len(p.accounts))
	for _, a := range p.accounts {
		p.balance[a.ID] = a.Balance
	}
	p.services = append([]Service(nil), s.Services...)
	sort.Slice(p.services, func(i, j int) bool { return p.services[i].ID < p.services[j].ID })
	for i := range p.services {
		if p.services[i].Status != "deleted" {
			p.live = append(p.live, i)
		}
	}

	out := make([]StreamEvent, 0, n)
	for len(out) < n {
		out = append(out, p.next(p.rng.Intn(100)))
	}
	return out
}

// planner ведёт состояние генератора: копию услуг и счетов, по которой проверяется применимость.
type planner struct {
	rng      *rand.Rand
	asOf     time.Time
	accounts []Account
	balance  map[int64]int64
	services []Service
	live     []int // индексы услуг, которые ещё не удалены
}

// pick возвращает первую подходящую услугу, начиная со случайной.
func (p *planner) pick(ok func(*Service) bool) *Service {
	if len(p.live) == 0 {
		return nil
	}
	start := p.rng.Intn(len(p.live))
	for k := range p.live {
		v := &p.services[p.live[(start+k)%len(p.live)]]
		if ok(v) {
			return v
		}
	}
	return nil
}

// next выбирает вид события по броску 0..99. Невозможный сейчас вид (остановить нечего: активных
// услуг не осталось) заменяется возобновлением, потом переключением автопродления, потом
// пополнением — так услуги не вымываются из активных и смесь держится весь прогон.
func (p *planner) next(roll int) StreamEvent {
	var ev *StreamEvent
	switch {
	case roll < shareTopUp:
		ev = p.topUp()
	case roll < shareTopUp+shareAutoprolog:
		ev = p.toggleAutoprolong()
	case roll < shareTopUp+shareAutoprolog+shareStop:
		if p.rng.Intn(2) == 0 {
			ev = p.resume()
		}
		if ev == nil {
			ev = p.stop()
		}
	case roll < shareTopUp+shareAutoprolog+shareStop+shareFailed:
		ev = p.renewalFailed()
	default:
		ev = p.prolongWait()
	}
	for _, fallback := range []func() *StreamEvent{p.resume, p.toggleAutoprolong, p.topUp} {
		if ev != nil {
			break
		}
		ev = fallback()
	}
	return *ev
}

func (p *planner) toggleAutoprolong() *StreamEvent {
	v := p.pick(func(v *Service) bool { return v.Status == "active" || v.Status == "prolong_wait" })
	if v == nil {
		return nil
	}
	v.AutoProlong = !v.AutoProlong
	kind := EventAutoprolongOff
	if v.AutoProlong {
		kind = EventAutoprolongOn
	}
	on := v.AutoProlong
	return &StreamEvent{Kind: kind, ServiceID: v.ID, AccountID: v.OwnerID, AutoProlong: &on}
}

// resume возобновляет остановленную услугу, а также ждавшую продления: её оплатили, срок идёт заново.
func (p *planner) resume() *StreamEvent {
	v := p.pick(func(v *Service) bool { return v.Status == "suspended" || v.Status == "prolong_wait" })
	if v == nil {
		return nil
	}
	ev := &StreamEvent{Kind: EventServiceResume, ServiceID: v.ID, AccountID: v.OwnerID}
	if v.Status == "prolong_wait" {
		v.ExpiresAt = p.asOf.Add(termDuration(v.PaymentTerm))
		exp := v.ExpiresAt
		ev.ExpiresAt = &exp
	}
	v.Status = "active"
	st := v.Status
	ev.Status = &st
	return ev
}

func (p *planner) stop() *StreamEvent {
	v := p.pick(func(v *Service) bool { return v.Status == "active" })
	if v == nil {
		return nil
	}
	v.Status = "suspended"
	st := v.Status
	return &StreamEvent{Kind: EventServiceStop, ServiceID: v.ID, AccountID: v.OwnerID, Status: &st}
}

// renewalFailed — неудачное продление: срок вышел, автопродление включено, а денег на счёте не хватило.
func (p *planner) renewalFailed() *StreamEvent {
	v := p.pick(func(v *Service) bool { return v.Status == "active" && v.AutoProlong && v.Price > 0 })
	if v == nil {
		return nil
	}
	v.Status = "prolong_wait"
	v.ExpiresAt = p.lapsedAt(*v)
	st, exp := v.Status, v.ExpiresAt
	ev := &StreamEvent{Kind: EventRenewalFailed, ServiceID: v.ID, AccountID: v.OwnerID, Status: &st, ExpiresAt: &exp}
	if p.balance[v.OwnerID] >= v.Price {
		low := v.Price - 1
		p.balance[v.OwnerID] = low
		ev.Balance = &low
	}
	return ev
}

func (p *planner) prolongWait() *StreamEvent {
	v := p.pick(func(v *Service) bool { return v.Status == "active" })
	if v == nil {
		return nil
	}
	v.Status = "prolong_wait"
	v.ExpiresAt = p.lapsedAt(*v)
	st, exp := v.Status, v.ExpiresAt
	return &StreamEvent{Kind: EventProlongWait, ServiceID: v.ID, AccountID: v.OwnerID, Status: &st, ExpiresAt: &exp}
}

// lapsedAt — срок услуги, истёкший на 1..48 часов раньше модельного времени, но не раньше создания.
func (p *planner) lapsedAt(v Service) time.Time {
	t := p.asOf.Add(-time.Duration(1+p.rng.Intn(48)) * time.Hour)
	if t.Before(v.CreatedAt) {
		t = v.CreatedAt
	}
	return t
}

// topUp — пополнение случайного счёта на 5..200 евро.
func (p *planner) topUp() *StreamEvent {
	a := p.accounts[p.rng.Intn(len(p.accounts))]
	add := int64(500 + p.rng.Intn(19501))
	next := p.balance[a.ID] + add
	if next > maxMoneyCents {
		next = maxMoneyCents
	}
	p.balance[a.ID] = next
	return &StreamEvent{Kind: EventTopUp, AccountID: a.ID, Balance: &next}
}

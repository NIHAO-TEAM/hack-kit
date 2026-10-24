package analytics

import (
	"context"
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
)

// TopicRecalculated — тема шины: после пересчёта в неё уходит номер версии результата (int64).
const TopicRecalculated = "recalculated"

// Computed — итог одного расчёта: результат и версии входов, по которым он получен.
type Computed[R any] struct {
	Result          R
	DataVersion     int64
	SettingsVersion int64
	Stages          Stages
	// Customers — сколько клиентов пересчитано в этом прогоне; Partial — пересчитаны только затронутые.
	Customers int
	Partial   bool
	// Extra — то, что частичный пересчёт собрал заранее (например, портфель); читает сервер.
	Extra any
}

// Stages — из чего сложилось время пересчёта: чтение снимка (или только его версии), перевод
// во входные данные ядра и сам расчёт.
type Stages struct {
	Load    time.Duration
	Convert time.Duration
	Score   time.Duration
}

// Calculation — последний результат с метаданными. Version растёт на 1 с каждым успешным
// пересчётом; DataVersion и SettingsVersion — версии снимка и настроек, по которым он считан.
type Calculation[R any] struct {
	Computed[R]
	Version      int64
	CalculatedAt time.Time
	Duration     time.Duration
}

// ComputeFunc читает снимок и настройки и считает модель. Её собирает сервер: модуль
// analytics не знает ни источника данных, ни хранилища настроек, ни ядра модели.
type ComputeFunc[R any] func(ctx context.Context) (Computed[R], error)

// Engine — один воркер пересчёта. Все события, из-за которых результат устарел, взводят флаг
// «грязно»: пачка событий даёт один пересчёт, а не очередь. Идущий пересчёт новое событие отменяет
// через context: его результат уже устарел, и он ничего не публикует. Отмена склеивается: прогон,
// начатый после отменённого, не отменяется, а доходит до конца (иначе поток событий не давал бы
// ни одному прогону завершиться); события за время такого прогона дают следующий. Результат
// держится в памяти и читается без блокировок.
type Engine[R any] struct {
	compute ComputeFunc[R]
	bus     *events.Bus
	clk     clock.Clock
	topics  []string

	dirty   atomic.Bool
	signal  chan struct{}
	latest  atomic.Pointer[Calculation[R]]
	version int64 // меняет только воркер
	errors  atomic.Int64
	lastErr atomic.Pointer[error] // ошибка последнего завершённого прогона; nil — прогон удался

	cancelled atomic.Int64

	mu        sync.Mutex
	cancelRun context.CancelFunc // отмена идущего прогона; nil, пока прогона нет
	protected bool               // идущий прогон начат после отменённого: его не отменяем

	partial  *Partial[R]
	observer func(*Calculation[R])
	pending  pending   // что накопилось с начала идущего прогона; под mu
	lastFull time.Time // начало последнего успешного полного пересчёта; меняет только воркер
}

// Partial включает частичный пересчёт: событие с известными клиентами пересчитывает только их.
type Partial[R any] struct {
	// Touched достаёт из события клиентов и версию данных, после которой они изменились. Пустой
	// список — «всё»: пересчёт полный.
	Touched func(topic string, payload any) (ids []int64, dataVersion int64)
	// Compute считает клиентов ids поверх prev. dataVersion — наибольшая версия данных из событий:
	// все изменения до неё уже в ids. version — номер, который получит результат.
	// ErrNeedFull — частичный пересчёт невозможен: воркер тут же считает полный.
	Compute func(ctx context.Context, prev *Calculation[R], ids []int64, dataVersion, version int64) (Computed[R], error)
	// FullEvery — как часто делать полный пересчёт, даже если события частичные (сверка накопленной
	// разности с нуля); 0 — каждый раз полный.
	FullEvery time.Duration
	// MaxShare — доля клиентов, выше которой пересчёт полный (число клиентов в последнем результате
	// берёт Total); 0 — без предела.
	MaxShare float64
	Total    func(r R) int
}

// ErrNeedFull — Partial.Compute отказался: нужен полный пересчёт.
var ErrNeedFull = errors.New("analytics: нужен полный пересчёт")

// pending — события, из-за которых результат устарел: либо «всё», либо набор клиентов.
type pending struct {
	full        bool
	ids         map[int64]struct{}
	dataVersion int64 // наибольшая версия данных среди событий с клиентами
}

func (p *pending) merge(o pending) {
	p.full = p.full || o.full
	p.dataVersion = max(p.dataVersion, o.dataVersion)
	if len(o.ids) == 0 {
		return
	}
	if p.ids == nil {
		p.ids = make(map[int64]struct{}, len(o.ids))
	}
	for id := range o.ids {
		p.ids[id] = struct{}{}
	}
}

// WithPartial включает частичный пересчёт; вызывать до Start.
func (e *Engine[R]) WithPartial(p Partial[R]) *Engine[R] {
	e.partial = &p
	e.pending.full = true
	return e
}

// WithObserver задаёт функцию, которую воркер вызывает после каждого успешного пересчёта (метрики); до Start.
func (e *Engine[R]) WithObserver(f func(*Calculation[R])) *Engine[R] {
	e.observer = f
	return e
}

// NewEngine создаёт воркер. topics — темы шины, после которых результат устарел
// (source.changed, model.changed).
func NewEngine[R any](compute ComputeFunc[R], bus *events.Bus, clk clock.Clock, topics ...string) *Engine[R] {
	return &Engine[R]{compute: compute, bus: bus, clk: clk, topics: topics, signal: make(chan struct{}, 1)}
}

// Latest — последний успешный результат; nil, пока ни один расчёт не удался.
func (e *Engine[R]) Latest() *Calculation[R] { return e.latest.Load() }

// Errors — сколько расчётов закончилось ошибкой (для метрик).
func (e *Engine[R]) Errors() int64 { return e.errors.Load() }

// LastError — ошибка последнего завершённого (не отменённого) прогона; nil, если он удался или прогонов ещё не было.
func (e *Engine[R]) LastError() error {
	if p := e.lastErr.Load(); p != nil {
		return *p
	}
	return nil
}

// Cancelled — сколько пересчётов отменено новым событием (для метрик).
func (e *Engine[R]) Cancelled() int64 { return e.cancelled.Load() }

// Start подписывается на события и запускает воркер; первый пересчёт идёт сразу. Подписка
// оформляется до возврата, поэтому событие после Start не потеряется. Воркер работает, пока
// не отменён ctx; done закрывается, когда он остановился.
func (e *Engine[R]) Start(ctx context.Context) (done <-chan struct{}) {
	var wg sync.WaitGroup
	cancels := make([]func(), 0, len(e.topics))
	for _, topic := range e.topics {
		ch, cancel := e.bus.Subscribe(topic)
		cancels = append(cancels, cancel)
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ev := range ch {
				e.note(ev.Topic, ev.Payload)
				e.markDirty()
			}
		}()
	}
	e.mu.Lock()
	e.pending.full = true // первый расчёт полный
	e.mu.Unlock()
	e.markDirty()

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		defer wg.Wait()
		defer func() {
			for _, cancel := range cancels {
				cancel()
			}
		}()
		for {
			select {
			case <-ctx.Done():
				return
			case <-e.signal:
			}
			for e.dirty.Swap(false) {
				e.recalc(ctx)
				if ctx.Err() != nil {
					return
				}
			}
		}
	}()
	return stopped
}

// note запоминает, кого коснулось событие: клиентов из полезной нагрузки или «всё».
func (e *Engine[R]) note(topic string, payload any) {
	if e.partial == nil {
		return
	}
	var ids []int64
	var dataVersion int64
	if e.partial.Touched != nil {
		ids, dataVersion = e.partial.Touched(topic, payload)
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(ids) == 0 {
		e.pending.full = true
		return
	}
	e.pending.dataVersion = max(e.pending.dataVersion, dataVersion)
	if e.pending.ids == nil {
		e.pending.ids = make(map[int64]struct{}, len(ids))
	}
	for _, id := range ids {
		e.pending.ids[id] = struct{}{}
	}
}

func (e *Engine[R]) markDirty() {
	e.dirty.Store(true)
	select {
	case e.signal <- struct{}{}:
	default:
	}
	e.mu.Lock()
	if e.cancelRun != nil && !e.protected {
		e.cancelRun()
	}
	e.mu.Unlock()
}

// recalc считает один раз. Ошибка не стирает прежний результат: она идёт в лог и счётчик.
func (e *Engine[R]) recalc(ctx context.Context) {
	started := e.clk.Now()
	runCtx, cancel := context.WithCancel(ctx)
	e.mu.Lock()
	e.cancelRun = cancel
	taken := e.pending // всё, что накопилось до начала прогона; новые события копятся в пустой pending
	e.pending = pending{}
	e.mu.Unlock()

	prev := e.latest.Load()
	c, err := e.run(runCtx, prev, taken, started)

	e.mu.Lock()
	e.cancelRun = nil
	superseded := runCtx.Err() != nil && ctx.Err() == nil
	e.protected = superseded // следующий прогон после отменённого не отменяем
	if superseded || err != nil {
		e.pending.merge(taken) // результат не принят: то, что он должен был учесть, остаётся за следующим
	}
	e.mu.Unlock()
	cancel()
	if superseded {
		// Данные уже устарели: ни результат, ни ошибка отменённого прогона не публикуются.
		e.cancelled.Add(1)
		log.Printf("analytics: пересчёт отменён новым событием через %s", e.clk.Now().Sub(started).Round(time.Millisecond))
		return
	}
	if err != nil {
		e.errors.Add(1)
		e.lastErr.Store(&err)
		log.Printf("analytics: пересчёт: %v", err)
		return
	}
	e.lastErr.Store(nil)
	e.version++
	if !c.Partial {
		e.lastFull = started
	}
	mode := "полный"
	if c.Partial {
		mode = "частичный"
	}
	log.Printf("analytics: пересчёт %d (%s, клиентов %d): чтение %s, перевод %s, расчёт %s, всего %s", e.version, mode, c.Customers,
		c.Stages.Load.Round(time.Millisecond), c.Stages.Convert.Round(time.Millisecond),
		c.Stages.Score.Round(time.Millisecond), e.clk.Now().Sub(started).Round(time.Millisecond))
	calc := &Calculation[R]{
		Computed:     c,
		Version:      e.version,
		CalculatedAt: started,
		Duration:     e.clk.Now().Sub(started),
	}
	e.latest.Store(calc)
	if e.observer != nil {
		e.observer(calc)
	}
	e.bus.Publish(TopicRecalculated, e.version)
}

// run выбирает вид пересчёта. Частичный — когда есть прежний результат, все события назвали клиентов,
// их немного и полный пересчёт делался не слишком давно; отказ частичного (ErrNeedFull) ведёт в полный.
func (e *Engine[R]) run(ctx context.Context, prev *Calculation[R], taken pending, started time.Time) (Computed[R], error) {
	if p := e.partial; p != nil && prev != nil && !taken.full && len(taken.ids) > 0 &&
		p.FullEvery > 0 && started.Sub(e.lastFull) < p.FullEvery &&
		(p.MaxShare <= 0 || p.Total == nil || float64(len(taken.ids)) <= p.MaxShare*float64(p.Total(prev.Result))) {
		ids := make([]int64, 0, len(taken.ids))
		for id := range taken.ids {
			ids = append(ids, id)
		}
		c, err := p.Compute(ctx, prev, ids, taken.dataVersion, e.version+1)
		if !errors.Is(err, ErrNeedFull) {
			c.Partial = err == nil
			return c, err
		}
	}
	return e.compute(ctx)
}

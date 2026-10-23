package analytics

import (
	"context"
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

// Engine — один воркер пересчёта. Все события, из-за которых результат устарел, только
// взводят флаг «грязно»: пачка событий даёт один пересчёт, а не очередь. Результат держится
// в памяти и читается без блокировок.
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
			for range ch {
				e.markDirty()
			}
		}()
	}
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

func (e *Engine[R]) markDirty() {
	e.dirty.Store(true)
	select {
	case e.signal <- struct{}{}:
	default:
	}
}

// recalc считает один раз. Ошибка не стирает прежний результат: она идёт в лог и счётчик.
func (e *Engine[R]) recalc(ctx context.Context) {
	started := e.clk.Now()
	c, err := e.compute(ctx)
	if err != nil {
		e.errors.Add(1)
		log.Printf("analytics: пересчёт: %v", err)
		return
	}
	e.version++
	e.latest.Store(&Calculation[R]{
		Computed:     c,
		Version:      e.version,
		CalculatedAt: started,
		Duration:     e.clk.Now().Sub(started),
	})
	e.bus.Publish(TopicRecalculated, e.version)
}

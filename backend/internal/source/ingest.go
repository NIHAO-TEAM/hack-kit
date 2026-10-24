package source

import (
	"context"
	"runtime/metrics"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// pipeBuffer — размер буфера канала между этапами конвейера. Пока писатель ждёт базу, буферы
// заполняются, отправитель блокируется и разбор тела встаёт: память не растёт вместе со снимком.
const pipeBuffer = 2048

// BodyError — тело не JSON и не форма снимка (400).
type BodyError struct{ Err error }

func (e *BodyError) Error() string { return e.Err.Error() }
func (e *BodyError) Unwrap() error { return e.Err }

type itemKind uint8

const (
	itemBegin itemKind = iota // начался массив tbl
	itemRow                   // запись массива tbl
	itemEnd                   // массив tbl закончился
)

// item — запись или метка массива в канале конвейера; одна структура на все три таблицы.
type item struct {
	kind itemKind
	tbl  table
	idx  int // место записи в своём массиве
	acc  Account
	svc  Service
	tx   Transaction
}

// dependsOn — таблица, строки которой нужны внешнему ключу (accounts ни от чего не зависит).
func dependsOn(t table) (table, bool) {
	switch t {
	case tableServices:
		return tableAccounts, true
	case tableTransactions:
		return tableServices, true
	}
	return 0, false
}

// sender шлёт записи в канал и считает время, потраченное на ожидание места (обратное давление).
type sender struct {
	ctx     context.Context
	ch      chan<- item
	blocked time.Duration
}

func (s *sender) send(it item) error {
	select {
	case s.ch <- it:
		return nil
	default:
	}
	start := time.Now()
	defer func() { s.blocked += time.Since(start) }()
	select {
	case s.ch <- it:
		return nil
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

// sendHandler — streamHandler, отдающий разобранные записи в первый канал конвейера.
type sendHandler struct{ sender }

func (h *sendHandler) begin(t table) error { return h.send(item{kind: itemBegin, tbl: t}) }
func (h *sendHandler) end(t table) error   { return h.send(item{kind: itemEnd, tbl: t}) }
func (h *sendHandler) account(i int, a Account) error {
	return h.send(item{kind: itemRow, tbl: tableAccounts, idx: i, acc: a})
}
func (h *sendHandler) service(i int, v Service) error {
	return h.send(item{kind: itemRow, tbl: tableServices, idx: i, svc: v})
}
func (h *sendHandler) transaction(i int, x Transaction) error {
	return h.send(item{kind: itemRow, tbl: tableTransactions, idx: i, tx: x})
}

// validateStage читает записи первого канала, проверяет и передаёт дальше.
type validateStage struct {
	v      *validator
	latest latest
	busy   time.Duration // время самой проверки, без ожиданий
	bad    *atomic.Bool
	sender sender
	in     <-chan item
	fatal  error
}

func (s *validateStage) run() {
	for it := range s.in {
		start := time.Now()
		switch it.kind {
		case itemRow:
			switch it.tbl {
			case tableAccounts:
				s.v.account(it.idx, it.acc)
			case tableServices:
				s.v.service(it.idx, it.svc)
				s.latest.service(it.svc)
			default:
				s.v.transaction(it.idx, it.tx)
				s.latest.transaction(it.tx)
			}
		case itemEnd:
			s.v.endTable(it.tbl)
		}
		if len(s.v.e.list) > 0 {
			s.bad.Store(true)
		}
		s.busy += time.Since(start)
		if s.fatal != nil {
			continue // отправлять некуда: дочитываем вход, чтобы разбор не завис на отправке
		}
		if err := s.sender.send(it); err != nil {
			s.fatal = err
		}
	}
}

// ingestPeakHeap — пик кучи за последнюю загрузку снимка (байты живых и ещё не собранных объектов).
var ingestPeakHeap = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "churn_snapshot_ingest_peak_heap_bytes",
	Help: "Пик кучи процесса за последнюю загрузку снимка (байты, по выборке каждые 10 мс).",
})

// peakSampler следит за кучей, пока идёт загрузка.
type peakSampler struct {
	start, peak uint64
	stop        chan struct{}
	done        chan struct{}
}

func heapObjects() uint64 {
	s := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	metrics.Read(s)
	if s[0].Value.Kind() != metrics.KindUint64 {
		return 0
	}
	return s[0].Value.Uint64()
}

func startPeakSampler() *peakSampler {
	p := &peakSampler{start: heapObjects(), stop: make(chan struct{}), done: make(chan struct{})}
	p.peak = p.start
	go func() {
		defer close(p.done)
		t := time.NewTicker(10 * time.Millisecond)
		defer t.Stop()
		for {
			if h := heapObjects(); h > p.peak {
				p.peak = h
			}
			select {
			case <-p.stop:
				return
			case <-t.C:
			}
		}
	}()
	return p
}

// finish останавливает выборку, пишет пик в метрику и возвращает его.
func (p *peakSampler) finish() uint64 {
	close(p.stop)
	<-p.done
	if h := heapObjects(); h > p.peak {
		p.peak = h
	}
	ingestPeakHeap.Set(float64(p.peak))
	return p.peak
}

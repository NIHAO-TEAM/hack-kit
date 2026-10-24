package source

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"example.com/app/internal/platform/api/gen"
)

// TopicStreamProgress — тема шины: ход потока событий, полезная нагрузка gen.StreamState.
// Значение то же, что у analytics.TopicStreamProgress: модули друг друга не импортируют.
const TopicStreamProgress = "source.stream"

// Границы и умолчания запроса потока.
const (
	streamMinEvents     = 100
	streamMaxEvents     = 100000
	streamDefaultEvents = 2500
	streamMaxRate       = 20000
	streamDefaultRate   = 100
	streamDefaultSeed   = 1
)

// streamRunTimeout — предел одного прогона; streamSettleTimeout — сколько ждём, пока пересчёт
// догонит итоговый снимок.
const (
	streamRunTimeout    = 15 * time.Minute
	streamSettleTimeout = 60 * time.Second
	streamPollEvery     = 2 * time.Millisecond
)

// StreamView — то, что поток берёт у пересчёта: последний готовый результат или результат
// «с нуля» по заданному снимку. Модуль не знает ядра модели, поэтому сборка сервера подставляет
// реализацию (WithStreamObserver).
type StreamView struct {
	// DataVersion — версия снимка, по которому считан результат; у Fresh 0.
	DataVersion int64
	// Recalcs — счётчик выполненных пересчётов движка; у Fresh 0.
	Recalcs int64
	// Levels — клиенты по уровням (high, medium, low, unscored, lost).
	Levels            map[string]int
	ExpectedLossCents int64
	ExposureCents     int64
	LostCents         int64
	// Customers — уровень каждого клиента по номеру счёта.
	Customers map[int64]string
}

// StreamObserver даёт потоку чтение результата пересчёта.
type StreamObserver interface {
	// StreamLatest — последний готовый результат; ok=false, пока ни одного пересчёта не было.
	StreamLatest() (StreamView, bool)
	// StreamFresh считает модель над снимком с нуля, не трогая состояние движка.
	StreamFresh(ctx context.Context, s Snapshot) (StreamView, error)
}

// WithStreamObserver подключает пересчёт к потоку событий; без него запуск отвечает 501.
func (a *SourceAPI) WithStreamObserver(o StreamObserver) *SourceAPI {
	a.observer = o
	return a
}

var (
	streamEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "churn_emulator_stream_events_total",
		Help: "События потока эмулятора; result: applied — применено, failed — не применено (штуки).",
	}, []string{"result"})
	streamApply = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "churn_emulator_stream_apply_seconds",
		Help:    "Время применения одного события потока эмулятора: от постановки в очередь до фиксации пачки (секунды).",
		Buckets: []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
	})
)

// StreamCollectors — метрики потока событий для /metrics.
func StreamCollectors() []prometheus.Collector {
	return []prometheus.Collector{streamEvents, streamApply, ingestBatchSize, ingestBatchSeconds, ingestQueueDepth}
}

// streamRun — состояние прогона; один на SourceAPI.
type streamRun struct {
	mu    sync.Mutex
	state gen.StreamState
}

func newStreamRun() *streamRun { return &streamRun{state: gen.StreamState{State: gen.Idle}} }

func (r *streamRun) get() gen.StreamState {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.state
}

// tryStart занимает прогон; false, если другой ещё идёт.
func (r *streamRun) tryStart(st gen.StreamState) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state.State == gen.Running {
		return false
	}
	r.state = st
	return true
}

func (r *streamRun) update(f func(*gen.StreamState)) gen.StreamState {
	r.mu.Lock()
	defer r.mu.Unlock()
	f(&r.state)
	return r.state
}

// Ошибки запуска потока.
var (
	errStreamBusy  = errors.New("другой прогон потока ещё идёт")
	errStreamNoRec = errors.New("пересчёт не подключён: поток событий недоступен")
)

// validateStream проверяет параметры запроса и подставляет умолчания.
func validateStream(req gen.StreamRequest) (n, rate int, seed int64, err error) {
	n, rate, seed = streamDefaultEvents, streamDefaultRate, streamDefaultSeed
	if req.Events != nil {
		n = *req.Events
	}
	if req.RatePerSecond != nil {
		rate = *req.RatePerSecond
	}
	if req.Seed != nil {
		seed = *req.Seed
	}
	if n < streamMinEvents || n > streamMaxEvents {
		return 0, 0, 0, badRequest{fmt.Errorf("событий должно быть от %d до %d", streamMinEvents, streamMaxEvents)}
	}
	if rate < 0 || rate > streamMaxRate {
		return 0, 0, 0, badRequest{fmt.Errorf("скорость должна быть от 0 до %d событий в секунду", streamMaxRate)}
	}
	return n, rate, seed, nil
}

// startStream занимает прогон и запускает его в своей горутине; возвращает номер прогона.
func (a *SourceAPI) startStream(ctx context.Context, n, rate int, seed int64) (string, error) {
	if a.observer == nil {
		return "", errStreamNoRec
	}
	runID := uuid.NewString()
	started := a.clk.Now()
	st := gen.StreamState{State: gen.Running, RunId: &runID, Events: n, RatePerSecond: &rate, Seed: &seed, StartedAt: &started}
	if !a.stream.tryStart(st) {
		return "", errStreamBusy
	}
	base, err := a.streamBase(ctx)
	if err == nil {
		err = a.ensureSnapshot(ctx)
	}
	if err != nil {
		a.stream.update(func(s *gen.StreamState) { *s = gen.StreamState{State: gen.Idle} })
		log.Printf("source: поток, чтение снимка: %v", err)
		return "", httpError{"снимок не прочитан"}
	}
	go a.runStream(runID, GenerateStream(base, n, seed), n, rate, base)
	return runID, nil
}

// streamBase — снимок, над которым строится поток; в пустой базе — сид, как у шагов эмулятора.
func (a *SourceAPI) streamBase(ctx context.Context) (Snapshot, error) {
	s, _, err := NewReader(a.pool).Load(ctx)
	if errors.Is(err, ErrNoSnapshot) {
		fresh, rerr := resetSnapshot()
		if rerr != nil {
			return Snapshot{}, rerr
		}
		fresh.AsOf = &seedAsOf
		return *fresh, nil
	}
	return s, err
}

// ensureSnapshot записывает сид, если в базе ещё нет снимка: точечные правки потока ложатся на
// существующие строки и версию, а в пустой базе им нечего править.
func (a *SourceAPI) ensureSnapshot(ctx context.Context) error {
	if _, ok, err := currentStatus(ctx, a.pool); err != nil || ok {
		return err
	}
	_, _, err := a.commitStep(ctx, func(s *Snapshot) (*Snapshot, string, error) { return s, OriginEmulator, nil })
	return err
}

// applyStreamEvent применяет событие теми же patchService и patchAccount, что PATCH-вызовы.
func applyStreamEvent(s *Snapshot, ev StreamEvent) error {
	if ev.Balance != nil {
		if _, err := patchAccount(s, ev.AccountID, gen.AccountPatch{Balance: ev.Balance}); err != nil {
			return err
		}
	}
	if ev.Status != nil || ev.AutoProlong != nil || ev.ExpiresAt != nil {
		p := gen.ServicePatch{AutoProlong: ev.AutoProlong, ExpiresAt: ev.ExpiresAt}
		if ev.Status != nil {
			st := gen.ServiceStatus(*ev.Status)
			p.Status = &st
		}
		if _, err := patchService(s, ev.ServiceID, p); err != nil {
			return err
		}
	}
	return nil
}

// pendingLag — события, применённые, но ещё не учтённые очередью.
type pendingLag struct {
	mu      sync.Mutex
	pending []pendingEvent
	lags    []time.Duration
}

type pendingEvent struct {
	version int64
	at      time.Time
}

func (p *pendingLag) add(version int64, at time.Time) {
	p.mu.Lock()
	p.pending = append(p.pending, pendingEvent{version, at})
	p.mu.Unlock()
}

// settle засчитывает задержку всем событиям, версия которых уже вошла в результат пересчёта.
func (p *pendingLag) settle(dataVersion int64, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	keep := p.pending[:0]
	for _, e := range p.pending {
		if e.version <= dataVersion {
			p.lags = append(p.lags, now.Sub(e.at))
		} else {
			keep = append(keep, e)
		}
	}
	p.pending = keep
}

// runStream ставит события в очередь приёма, которая пишет их пачками, и собирает отчёт. Работает в своей горутине.
func (a *SourceAPI) runStream(runID string, events []StreamEvent, n, rate int, base Snapshot) {
	ctx, cancel := context.WithTimeout(context.Background(), streamRunTimeout)
	defer cancel()

	fail := func(err error) {
		log.Printf("source: поток %s: %v", runID, err)
		msg := err.Error()
		st := a.stream.update(func(s *gen.StreamState) { s.State, s.Error = gen.Failed, &msg })
		a.bus.Publish(TopicStreamProgress, st)
	}

	before, err := a.observer.StreamFresh(ctx, base)
	if err != nil {
		fail(fmt.Errorf("расчёт до потока: %w", err))
		return
	}
	startRecalcs := int64(0)
	if v, ok := a.observer.StreamLatest(); ok {
		startRecalcs = v.Recalcs
	}

	lag := &pendingLag{}
	stopWatch := make(chan struct{})
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		t := time.NewTicker(streamPollEvery)
		defer t.Stop()
		for {
			select {
			case <-stopWatch:
				return
			case <-t.C:
				if v, ok := a.observer.StreamLatest(); ok {
					lag.settle(v.DataVersion, time.Now())
				}
			}
		}
	}()

	var (
		applyDur        = make([]time.Duration, 0, len(events))
		applied, failed int
		started         = time.Now()
		lastPublish     = started
	)
	progress := func(force bool) {
		if !force && time.Since(lastPublish) < 150*time.Millisecond {
			return
		}
		lastPublish = time.Now()
		done := applied + failed
		st := a.stream.update(func(s *gen.StreamState) { s.Applied = done })
		a.bus.Publish(TopicStreamProgress, st)
	}
	// onBatch вызывает писатель из своей горутины, пока отправитель ждёт в submit или на паузе;
	// счётчики читает только эта же горутина, а после finish — runStream.
	ing := newEventWriter(a.pool, newIngestState(&base), time.Now,
		func(ch Change) { a.bus.Publish(TopicChanged, ch) },
		func(res []ingestResult) {
			now := time.Now()
			for _, r := range res {
				applyDur = append(applyDur, r.dur)
				streamApply.Observe(r.dur.Seconds())
				if r.err != nil {
					failed++
					streamEvents.WithLabelValues("failed").Inc()
					log.Printf("source: поток %s: событие %d не применено: %v", runID, r.seq, r.err)
					continue
				}
				applied++
				streamEvents.WithLabelValues("applied").Inc()
				lag.add(r.version, now)
			}
			progress(false)
		})
	ing.start(ctx)
	var gap time.Duration
	if rate > 0 {
		gap = time.Second / time.Duration(rate)
	}
	for i, ev := range events {
		if gap > 0 {
			if wait := time.Until(started.Add(time.Duration(i) * gap)); wait > 0 {
				select {
				case <-time.After(wait):
				case <-ctx.Done():
				}
			}
		}
		if !ing.submit(ctx, i, ev) || ctx.Err() != nil {
			ing.finish()
			close(stopWatch)
			<-watchDone
			fail(fmt.Errorf("прогон прерван: %w", ctx.Err()))
			return
		}
	}
	ing.finish()
	progress(true)
	elapsed := time.Since(started)

	after, cons, err := a.settleAndVerify(ctx)
	close(stopWatch)
	<-watchDone
	if err != nil {
		fail(err)
		return
	}
	lag.settle(after.DataVersion, time.Now())

	rep := gen.StreamReport{
		Events: len(events), Applied: applied, Failed: failed,
		DurationMs: elapsed.Milliseconds(), EventsPerSecond: float32(float64(applied+failed) / elapsed.Seconds()),
		ApplyP50Ms: ms(percentile(applyDur, 50)), ApplyP95Ms: ms(percentile(applyDur, 95)), ApplyMaxMs: ms(percentile(applyDur, 100)),
		Recalcs:      int(after.Recalcs - startRecalcs),
		LevelsBefore: levelCounts(before.Levels), LevelsAfter: levelCounts(after.Levels),
		LevelChanges:    levelChanges(before.Customers, after.Customers),
		LossBeforeCents: before.ExpectedLossCents, LossAfterCents: after.ExpectedLossCents,
		Consistency: cons,
	}
	if p95 := percentile(lag.lags, 95); len(lag.lags) > 0 {
		v := ms(p95)
		rep.QueueLagP95Ms = &v
	}
	st := a.stream.update(func(s *gen.StreamState) {
		s.State, s.Applied, s.Report = gen.Done, applied+failed, &rep
	})
	a.bus.Publish(TopicStreamProgress, st)
}

// settleAndVerify ждёт, пока пересчёт догонит итоговый снимок, и сверяет его результат с расчётом
// с нуля над тем же снимком.
func (a *SourceAPI) settleAndVerify(ctx context.Context) (StreamView, gen.StreamConsistency, error) {
	deadline := time.Now().Add(streamSettleTimeout)
	for time.Now().Before(deadline) && ctx.Err() == nil {
		snap, version, err := NewReader(a.pool).Load(ctx)
		if err != nil {
			return StreamView{}, gen.StreamConsistency{}, fmt.Errorf("итоговый снимок не прочитан: %w", err)
		}
		var cur StreamView
		for time.Now().Before(deadline) {
			if v, ok := a.observer.StreamLatest(); ok && v.DataVersion >= version {
				cur = v
				break
			}
			time.Sleep(streamPollEvery)
		}
		if cur.DataVersion != version {
			continue // за это время записан более новый снимок или пересчёт не успел: читаем заново
		}
		fresh, err := a.observer.StreamFresh(ctx, snap)
		if err != nil {
			return StreamView{}, gen.StreamConsistency{}, fmt.Errorf("расчёт с нуля: %w", err)
		}
		diff := maxAbs(cur.ExpectedLossCents-fresh.ExpectedLossCents, cur.ExposureCents-fresh.ExposureCents, cur.LostCents-fresh.LostCents)
		ok := diff == 0 && levelCounts(cur.Levels) == levelCounts(fresh.Levels)
		return cur, gen.StreamConsistency{Ok: ok, DiffCents: diff, Version: version}, nil
	}
	return StreamView{}, gen.StreamConsistency{}, errors.New("пересчёт не догнал итоговый снимок за отведённое время")
}

func maxAbs(v ...int64) int64 {
	var m int64
	for _, x := range v {
		if x < 0 {
			x = -x
		}
		if x > m {
			m = x
		}
	}
	return m
}

func ms(d time.Duration) float32 { return float32(d.Seconds() * 1000) }

// percentile — p-й процентиль способом ближайшего ранга; у пустого набора 0.
func percentile(d []time.Duration, p int) time.Duration {
	if len(d) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), d...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	rank := (p*len(s) + 99) / 100
	if rank < 1 {
		rank = 1
	}
	return s[rank-1]
}

func levelCounts(m map[string]int) gen.LevelCounts {
	return gen.LevelCounts{High: m["high"], Medium: m["medium"], Low: m["low"], Unscored: m["unscored"], Lost: m["lost"]}
}

// levelChanges считает клиентов, сменивших уровень, по парам «было → стало»; частые пары первыми.
func levelChanges(before, after map[int64]string) []gen.StreamLevelChange {
	type pair struct{ from, to string }
	counts := map[pair]int{}
	for id, to := range after {
		if from, ok := before[id]; ok && from != to {
			counts[pair{from, to}]++
		}
	}
	out := make([]gen.StreamLevelChange, 0, len(counts))
	for p, c := range counts {
		out = append(out, gen.StreamLevelChange{From: gen.RiskLevel(p.from), To: gen.RiskLevel(p.to), Count: c})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}

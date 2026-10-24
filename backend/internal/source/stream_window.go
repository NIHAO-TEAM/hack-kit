package source

import (
	"sync"
	"time"

	"example.com/app/internal/platform/api/gen"
)

// streamWindowSize — сколько секундных отсчётов держим в окне прогона.
const streamWindowSize = 60

// windowRecorder копит секундные отсчёты прогона. Писатель и отправитель зовут add*, раз в секунду
// runStream зовёт tick и получает окно целиком (не длиннее streamWindowSize, старые первыми).
type windowRecorder struct {
	mu         sync.Mutex
	second     int
	accepted   int
	applied    int
	failed     int
	batches    int
	batchSum   int
	lags       []time.Duration
	lastRecalc int64
	points     []gen.StreamWindowPoint
}

func newWindowRecorder(startRecalcs int64) *windowRecorder {
	return &windowRecorder{lastRecalc: startRecalcs}
}

func (w *windowRecorder) addAccepted() {
	w.mu.Lock()
	w.accepted++
	w.mu.Unlock()
}

// addBatch учитывает записанную пачку: сколько событий применено и сколько отказов.
func (w *windowRecorder) addBatch(applied, failed int) {
	w.mu.Lock()
	w.applied += applied
	w.failed += failed
	if applied > 0 {
		w.batches++
		w.batchSum += applied
	}
	w.mu.Unlock()
}

// addLags учитывает задержки событий, которые пересчёт засчитал за последнюю секунду.
func (w *windowRecorder) addLags(d []time.Duration) {
	if len(d) == 0 {
		return
	}
	w.mu.Lock()
	w.lags = append(w.lags, d...)
	w.mu.Unlock()
}

// tick закрывает секунду: queueDepth и recalcs — значения на её конец (recalcs — счётчик движка
// нарастающим итогом). Возвращает копию окна.
func (w *windowRecorder) tick(queueDepth int, recalcs int64) []gen.StreamWindowPoint {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.second++
	pt := gen.StreamWindowPoint{
		T: w.second, Accepted: w.accepted, Applied: w.applied, Failed: w.failed,
		QueueDepth: queueDepth, Recalcs: int(recalcs - w.lastRecalc),
	}
	if w.batches > 0 {
		pt.BatchSize = float32(w.batchSum) / float32(w.batches)
	}
	if len(w.lags) > 0 {
		pt.LagP95Ms = ms(percentile(w.lags, 95))
	}
	w.lastRecalc = recalcs
	w.accepted, w.applied, w.failed, w.batches, w.batchSum, w.lags = 0, 0, 0, 0, 0, w.lags[:0]
	w.points = append(w.points, pt)
	if len(w.points) > streamWindowSize {
		w.points = append(w.points[:0], w.points[len(w.points)-streamWindowSize:]...)
	}
	return append([]gen.StreamWindowPoint(nil), w.points...)
}

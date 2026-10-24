package source

import (
	"testing"
	"time"
)

func TestWindowRecorderTick(t *testing.T) {
	w := newWindowRecorder(10)
	for i := 0; i < 5; i++ {
		w.addAccepted()
	}
	w.addBatch(3, 1)
	w.addBatch(1, 0)
	w.addLags([]time.Duration{10 * time.Millisecond, 20 * time.Millisecond})
	pts := w.tick(7, 12)
	if len(pts) != 1 {
		t.Fatalf("отсчётов %d, ждали 1", len(pts))
	}
	p := pts[0]
	if p.T != 1 || p.Accepted != 5 || p.Applied != 4 || p.Failed != 1 || p.QueueDepth != 7 || p.Recalcs != 2 {
		t.Fatalf("отсчёт: %+v", p)
	}
	if p.BatchSize != 2 {
		t.Fatalf("размер пачки %v, ждали 2", p.BatchSize)
	}
	if p.LagP95Ms != 20 {
		t.Fatalf("задержка p95 %v, ждали 20", p.LagP95Ms)
	}
	// следующая секунда начинается с нуля
	p = w.tick(0, 12)[1]
	if p.T != 2 || p.Accepted != 0 || p.Applied != 0 || p.BatchSize != 0 || p.LagP95Ms != 0 || p.Recalcs != 0 {
		t.Fatalf("вторая секунда: %+v", p)
	}
}

func TestWindowRecorderKeeps60AndSums(t *testing.T) {
	w := newWindowRecorder(0)
	total := 0
	var pts int
	for s := 1; s <= 75; s++ {
		w.addBatch(s, 0)
		total += s
		pts = len(w.tick(0, 0))
	}
	if pts != streamWindowSize {
		t.Fatalf("окно %d, ждали %d", pts, streamWindowSize)
	}
	win := w.tick(0, 0)
	if win[0].T != 17 || win[len(win)-1].T != 76 {
		t.Fatalf("окно %d..%d", win[0].T, win[len(win)-1].T)
	}
	sum := 0
	for _, p := range win {
		sum += p.Applied
	}
	want := 0
	for s := 17; s <= 75; s++ {
		want += s
	}
	if sum != want {
		t.Fatalf("сумма %d, ждали %d", sum, want)
	}
	_ = total
}

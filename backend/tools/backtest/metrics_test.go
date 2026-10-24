package backtest

import (
	"bytes"
	"io"
	"math"
	"testing"
)

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

func TestROCAUC(t *testing.T) {
	pos := []bool{false, false, true, true}
	cases := []struct {
		name  string
		score []float64
		want  float64
	}{
		{"идеально", []float64{1, 2, 3, 4}, 1},
		{"наоборот", []float64{4, 3, 2, 1}, 0},
		{"все равны", []float64{1, 1, 1, 1}, 0.5},
		{"одна ошибка", []float64{1, 3, 2, 4}, 0.75},
	}
	for _, c := range cases {
		if got := ROCAUC(c.score, pos); !near(got, c.want) {
			t.Errorf("%s: %v, ждали %v", c.name, got, c.want)
		}
	}
	if !math.IsNaN(ROCAUC([]float64{1, 2}, []bool{true, true})) {
		t.Error("без отрицательных ждали NaN")
	}
}

func TestPRAUC(t *testing.T) {
	// идеально: точность 1 на каждом шаге
	if got := PRAUC([]float64{4, 3, 2, 1}, []bool{true, true, false, false}); !near(got, 1) {
		t.Errorf("идеально: %v", got)
	}
	// положительные на местах 1 и 3: (1/1 + 2/3)/2
	if got := PRAUC([]float64{4, 3, 2, 1}, []bool{true, false, true, false}); !near(got, (1+2.0/3)/2) {
		t.Errorf("1 и 3 места: %v", got)
	}
	// все равны: один шаг, точность = доля положительных
	if got := PRAUC([]float64{1, 1, 1, 1}, []bool{true, false, false, false}); !near(got, 0.25) {
		t.Errorf("все равны: %v", got)
	}
	if !math.IsNaN(PRAUC([]float64{1}, []bool{false})) {
		t.Error("без положительных ждали NaN")
	}
}

func TestTopShare(t *testing.T) {
	lost := []int64{0, 70, 0, 20, 10, 0, 0, 0, 0, 0}
	key := []float64{1, 10, 2, 9, 8, 3, 4, 5, 6, 7}
	ord := Order(key, []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	if got := TopShare(ord, lost, 0.10); !near(got, 0.7) {
		t.Errorf("верхние 10 %%: %v", got)
	}
	if got := TopShare(ord, lost, 0.30); !near(got, 1) {
		t.Errorf("верхние 30 %%: %v", got)
	}
	if !math.IsNaN(TopShare(ord, make([]int64, 10), 0.1)) {
		t.Error("без потерь ждали NaN")
	}
}

func TestOrderTieByID(t *testing.T) {
	ord := Order([]float64{1, 1, 1}, []int64{30, 10, 20})
	if ord[0] != 1 || ord[1] != 2 || ord[2] != 0 {
		t.Errorf("при равных ключах порядок по возрастанию id: %v", ord)
	}
}

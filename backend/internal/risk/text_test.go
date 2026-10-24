package risk

import (
	"fmt"
	"strconv"
	"testing"
)

// eurosRef — прежняя реализация через Sprintf: новая обязана давать тот же текст.
func eurosRef(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	whole := strconv.FormatInt(cents/100, 10)
	var b []byte
	for i := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b = append(b, ' ')
		}
		b = append(b, whole[i])
	}
	return fmt.Sprintf("%s%s,%02d €", sign, b, cents%100)
}

func TestEurosSameAsReference(t *testing.T) {
	for _, c := range []int64{0, 5, 99, 100, 101, 12345, 144490, 100000, 99999999, -1, -100, -144490, 123456789012} {
		if got, want := euros(c), eurosRef(c); got != want {
			t.Errorf("euros(%d) = %q, want %q", c, got, want)
		}
	}
}

func TestDaysText(t *testing.T) {
	for days, want := range map[float64]string{0.01: "1 ч", 0.5: "12 ч", 1: "1 дн.", 2.2: "3 дн.", 6.99: "7 дн."} {
		if got := daysText(days); got != want {
			t.Errorf("daysText(%v) = %q, want %q", days, got, want)
		}
	}
}

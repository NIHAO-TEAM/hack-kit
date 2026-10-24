package risk

import (
	"math"
	"strconv"
)

// daysText — «N дн.» или «N ч» для текстов причин.
func daysText(days float64) string {
	if days < 1 {
		return strconv.Itoa(int(math.Ceil(days*24))) + " ч"
	}
	return strconv.Itoa(int(math.Ceil(days))) + " дн."
}

// euros — центы в тексте причины: «1 444,90 €».
func euros(cents int64) string {
	var buf [32]byte
	b := buf[:0]
	if cents < 0 {
		b = append(b, '-')
		cents = -cents
	}
	var digits [20]byte
	whole := strconv.AppendInt(digits[:0], cents/100, 10)
	for i := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b = append(b, ' ')
		}
		b = append(b, whole[i])
	}
	b = append(b, ',', byte('0'+cents%100/10), byte('0'+cents%10))
	b = append(b, " €"...)
	return string(b)
}

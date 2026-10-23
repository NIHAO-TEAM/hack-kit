package risk

import (
	"fmt"
	"math"
	"strconv"
)

// daysText — «N дн.» или «N ч» для текстов причин.
func daysText(days float64) string {
	if days < 1 {
		return fmt.Sprintf("%d ч", int(math.Ceil(days*24)))
	}
	return fmt.Sprintf("%d дн.", int(math.Ceil(days)))
}

// euros — центы в тексте причины: «1 444,90 €».
func euros(cents int64) string {
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

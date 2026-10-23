package risk

import (
	"fmt"
	"math"
)

// daysText — «N дн.» или «N ч» для текстов причин.
func daysText(days float64) string {
	if days < 1 {
		return fmt.Sprintf("%d ч", int(math.Ceil(days*24)))
	}
	return fmt.Sprintf("%d дн.", int(math.Ceil(days)))
}

// eurosText — центы как «529,84» (запятая, без разделителя тысяч).
func eurosText(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s%d,%02d", sign, cents/100, cents%100)
}

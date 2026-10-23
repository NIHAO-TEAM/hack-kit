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

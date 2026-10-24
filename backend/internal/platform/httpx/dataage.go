package httpx

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// DataAge ставит заголовок X-Data-Age (целые секунды) на ответы чтения /api/: сколько прошло
// с последнего готового расчёта. Читатель видит, что при недоступной базе получил не свежие данные.
// calculatedAt возвращает время расчёта; нулевое — расчёта нет, заголовок не ставится.
func DataAge(calculatedAt func() time.Time, now func() time.Time, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/") {
			if at := calculatedAt(); !at.IsZero() {
				age := max(now().Sub(at), 0)
				w.Header().Set("X-Data-Age", strconv.FormatInt(int64(age/time.Second), 10))
			}
		}
		next.ServeHTTP(w, r)
	})
}

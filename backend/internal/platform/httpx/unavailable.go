package httpx

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgconn"
)

// retryAfterDB — через сколько секунд клиенту повторить запрос, пока база недоступна.
const retryAfterDB = "2"

// IsDBUnavailable сообщает, что ошибка — отказ соединения с базой (пауза, перезапуск, сеть),
// а не ошибка запроса: повтор позже может пройти. Ошибки данных и SQL сюда не входят.
func IsDBUnavailable(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// 08 — соединение, 57P01..03 — остановка и обрыв по команде админа, 53300 — нет слотов.
		c := pgErr.Code
		return strings.HasPrefix(c, "08") || strings.HasPrefix(c, "57P0") || c == "53300"
	}
	var conn *pgconn.ConnectError
	var netErr net.Error
	switch {
	case errors.As(err, &conn),
		errors.As(err, &netErr),
		errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF),
		errors.Is(err, syscall.ECONNREFUSED), errors.Is(err, syscall.ECONNRESET), errors.Is(err, syscall.EPIPE),
		errors.Is(err, context.DeadlineExceeded),
		pgconn.Timeout(err),
		strings.Contains(err.Error(), "closed pool"):
		return true
	}
	return false
}

// WriteUnavailable отвечает 503 с Retry-After: базы нет, запрос стоит повторить.
func WriteUnavailable(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Retry-After", retryAfterDB)
	WriteProblem(w, r, http.StatusServiceUnavailable, "база данных недоступна, повторите запрос позже")
}

// WriteFailure отвечает на ошибку хранилища: 503 с Retry-After, если база недоступна, иначе 500
// с переданным описанием. Ошибку пишет в лог.
func WriteFailure(w http.ResponseWriter, r *http.Request, err error, detail string) {
	if IsDBUnavailable(err) {
		WriteUnavailable(w, r)
		return
	}
	log.Printf("httpx: %s: %v", detail, err)
	WriteProblem(w, r, http.StatusInternalServerError, detail)
}

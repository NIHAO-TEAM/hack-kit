package httpx

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"syscall"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestIsDBUnavailable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"отказ соединения", fmt.Errorf("auth: %w", syscall.ECONNREFUSED), true},
		{"обрыв", fmt.Errorf("x: %w", io.ErrUnexpectedEOF), true},
		{"таймаут контекста", context.DeadlineExceeded, true},
		{"закрытый пул", errors.New("closed pool"), true},
		{"остановка сервера", &pgconn.PgError{Code: "57P01"}, true},
		{"соединение", &pgconn.PgError{Code: "08006"}, true},
		{"нарушение уникальности", &pgconn.PgError{Code: "23505"}, false},
		{"обычная ошибка", errors.New("ошибка"), false},
		{"отмена клиента", context.Canceled, false},
	}
	for _, c := range cases {
		if got := IsDBUnavailable(c.err); got != c.want {
			t.Errorf("%s: %v, нужно %v", c.name, got, c.want)
		}
	}
}

func TestWriteFailure(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteFailure(rec, httptest.NewRequest(http.MethodGet, "/", nil), syscall.ECONNREFUSED, "сбой")
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("база недоступна: %d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	rec = httptest.NewRecorder()
	WriteFailure(rec, httptest.NewRequest(http.MethodGet, "/", nil), errors.New("прочее"), "сбой")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("прочая ошибка: %d", rec.Code)
	}
}

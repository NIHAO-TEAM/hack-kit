// Package httpx — общая обвязка HTTP: ошибки application/problem+json (RFC 9457).
package httpx

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// FieldError описывает ошибку одного поля запроса.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Problem — тело ошибки по RFC 9457; correlation_id равен X-Request-Id.
type Problem struct {
	Type          string       `json:"type"`
	Title         string       `json:"title"`
	Status        int          `json:"status"`
	Detail        string       `json:"detail,omitempty"`
	CorrelationID string       `json:"correlation_id,omitempty"`
	Errors        []FieldError `json:"errors,omitempty"`
}

// WriteProblem пишет ошибку с нужным статусом. correlation_id берётся из заголовка ответа
// X-Request-Id (его выставляет withRequestID), иначе из заголовка запроса.
func WriteProblem(w http.ResponseWriter, r *http.Request, status int, detail string, errs ...FieldError) {
	id := w.Header().Get("X-Request-Id")
	if id == "" {
		id = r.Header.Get("X-Request-Id")
	}
	detail = truncateDetail(detail)
	p := Problem{
		Type:          "about:blank",
		Title:         http.StatusText(status),
		Status:        status,
		Detail:        detail,
		CorrelationID: id,
		Errors:        errs,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		log.Printf("httpx: запись problem: %v", err)
	}
}

// NotImplemented отвечает 501: модуль операции ещё не реализован.
func NotImplemented(w http.ResponseWriter, r *http.Request) {
	WriteProblem(w, r, http.StatusNotImplemented, "операция ещё не реализована")
}

// maxDetail — предел detail в байтах: ошибка разбора может цитировать присланное значение целиком.
const maxDetail = 1000

func truncateDetail(s string) string {
	if len(s) <= maxDetail {
		return s
	}
	return strings.ToValidUTF8(s[:maxDetail], "") + "…"
}

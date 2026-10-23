package auth_test

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"example.com/app/internal/apitest"
)

func TestLoginRateLimit(t *testing.T) {
	s := apitest.New(t)
	for i := 1; i <= 5; i++ {
		if resp := login(t, s, "jury", "wrong"); resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("попытка %d: статус %d", i, resp.StatusCode)
		}
		s.Clock.Advance(10 * time.Second)
	}

	// Шестая попытка блокируется, даже с верным паролем.
	resp := login(t, s, "jury", apitest.Password("jury"))
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("6-я попытка: статус %d, ждали 429", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/problem+json" {
		t.Errorf("Content-Type %q", resp.Header.Get("Content-Type"))
	}
	wait, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	if err != nil || wait <= 0 || wait > 300 {
		t.Errorf("Retry-After %q", resp.Header.Get("Retry-After"))
	}
	if sessionCookie(resp) != nil {
		t.Error("cookie выдана при блокировке")
	}

	// Другой логин с того же IP не заблокирован.
	if resp := login(t, s, "analyst", apitest.Password("analyst")); resp.StatusCode != http.StatusOK {
		t.Errorf("другой логин: статус %d", resp.StatusCode)
	}

	// Через 5 минут после первой неудачи окно освобождается.
	s.Clock.Advance(5 * time.Minute)
	if resp := login(t, s, "jury", apitest.Password("jury")); resp.StatusCode != http.StatusOK {
		t.Errorf("после окна: статус %d", resp.StatusCode)
	}
}

func TestLoginRateLimitKeyedByIPBehindTrustedProxyOnly(t *testing.T) {
	s := apitest.New(t)
	post := func(ip string) int {
		resp := loginWith(t, s, "jury", "wrong", func(r *http.Request) { r.Header.Set("X-Forwarded-For", ip) })
		return resp.StatusCode
	}
	// TRUSTED_PROXY выключен: X-Forwarded-For игнорируется, счётчик общий.
	for i := 0; i < 5; i++ {
		post("10.0.0." + strconv.Itoa(i))
	}
	if code := post("10.0.0.99"); code != http.StatusTooManyRequests {
		t.Errorf("подмена X-Forwarded-For обошла лимит: %d", code)
	}
}

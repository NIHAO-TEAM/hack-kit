//go:build integration

package source_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"example.com/app/internal/apitest"
	"example.com/app/internal/platform/api/gen"
)

func postNDJSON(t *testing.T, c *gen.ClientWithResponses, body string) *gen.PostSourceEventsResponse {
	t.Helper()
	res, err := c.PostSourceEventsWithBodyWithResponse(context.Background(), "application/x-ndjson", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func topUp(id string, account, balance int64) string {
	return fmt.Sprintf(`{"eventId":%q,"kind":"top_up","accountId":%d,"balance":%d}`, id, account, balance)
}

// Права: без сессии 401, маркетолог 403, остальные роли и токен источника — 202.
func TestPostSourceEventsRoles(t *testing.T) {
	s := apitest.New(t)
	if res := put(t, s.As(t, "analyst"), organizers(t)); res.JSON200 == nil {
		t.Fatalf("снимок: %s", res.Body)
	}
	var account int64
	if err := s.Pool.QueryRow(context.Background(), "SELECT id FROM accounts ORDER BY id LIMIT 1").Scan(&account); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		c    *gen.ClientWithResponses
		want int
	}{
		{"без сессии", s.Anonymous(t), http.StatusUnauthorized},
		{"маркетолог", s.As(t, "marketer"), http.StatusForbidden},
		{"аналитик", s.As(t, "analyst"), http.StatusAccepted},
		{"jury", s.As(t, "jury"), http.StatusAccepted},
		{"токен источника", s.WithBearer(t, apitest.SourceToken), http.StatusAccepted},
		{"токен метрик", s.WithBearer(t, apitest.MetricsToken), http.StatusUnauthorized},
	} {
		res := postNDJSON(t, tc.c, topUp("role-"+tc.name, account, 1000))
		if res.StatusCode() != tc.want {
			t.Errorf("%s: %d, ждали %d: %s", tc.name, res.StatusCode(), tc.want, res.Body)
		}
	}
}

// Пока снимка нет, событиям применяться не к чему: 400.
func TestPostSourceEventsNoSnapshot(t *testing.T) {
	s := apitest.New(t)
	res := postNDJSON(t, s.As(t, "analyst"), topUp("e1", 1, 100))
	if res.StatusCode() != http.StatusBadRequest {
		t.Fatalf("без снимка: %d %s", res.StatusCode(), res.Body)
	}
}

// Событие применяется асинхронно: баланс меняется, версия растёт, повтор eventId не применяется.
func TestPostSourceEventsAppliesOnceAndDeduplicates(t *testing.T) {
	s := apitest.New(t)
	c := s.As(t, "analyst")
	if res := put(t, c, organizers(t)); res.JSON200 == nil {
		t.Fatalf("снимок: %s", res.Body)
	}
	ctx := context.Background()
	var account, balance, version0 int64
	if err := s.Pool.QueryRow(ctx, "SELECT id, balance FROM accounts ORDER BY id LIMIT 1").Scan(&account, &balance); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(ctx, "SELECT max(version) FROM snapshots").Scan(&version0); err != nil {
		t.Fatal(err)
	}

	res := postNDJSON(t, c, topUp("dup-1", account, balance+777)+"\n"+`{"eventId":"bad"}`)
	if res.JSON202 == nil || res.JSON202.Accepted != 1 || res.JSON202.Duplicate != 0 || len(res.JSON202.Rejected) != 1 || res.JSON202.Rejected[0].Line != 2 {
		t.Fatalf("первый запрос: %d %s", res.StatusCode(), res.Body)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		var got, version int64
		if err := s.Pool.QueryRow(ctx, "SELECT (SELECT balance FROM accounts WHERE id = $1), (SELECT max(version) FROM snapshots)", account).Scan(&got, &version); err != nil {
			t.Fatal(err)
		}
		if got == balance+777 && version > version0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("событие не применено за 10 с: баланс %d (ждали %d), версия %d (была %d)", got, balance+777, version, version0)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Тот же eventId с другой суммой — дубль: баланс остаётся прежним.
	res = postNDJSON(t, c, topUp("dup-1", account, balance+1))
	if res.JSON202 == nil || res.JSON202.Accepted != 0 || res.JSON202.Duplicate != 1 {
		t.Fatalf("повтор: %d %s", res.StatusCode(), res.Body)
	}
	time.Sleep(100 * time.Millisecond)
	var got int64
	if err := s.Pool.QueryRow(ctx, "SELECT balance FROM accounts WHERE id = $1", account).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != balance+777 {
		t.Errorf("повтор eventId применён: баланс %d, ждали %d", got, balance+777)
	}
}

// Массив JSON принимается так же, как NDJSON.
func TestPostSourceEventsJSONArray(t *testing.T) {
	s := apitest.New(t)
	c := s.As(t, "analyst")
	if res := put(t, c, organizers(t)); res.JSON200 == nil {
		t.Fatalf("снимок: %s", res.Body)
	}
	var account int64
	if err := s.Pool.QueryRow(context.Background(), "SELECT id FROM accounts ORDER BY id LIMIT 1").Scan(&account); err != nil {
		t.Fatal(err)
	}
	body := "[" + topUp("arr-1", account, 10) + "," + topUp("arr-2", account, 20) + "]"
	res, err := c.PostSourceEventsWithBodyWithResponse(context.Background(), "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if res.JSON202 == nil || res.JSON202.Accepted != 2 {
		t.Fatalf("массив: %d %s", res.StatusCode(), res.Body)
	}
}

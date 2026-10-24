package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"example.com/app/internal/platform/clock"
)

var t0 = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

// h возвращает хэш-подобную строку: два первых hex-знака задают шард.
func h(i int) string { return fmt.Sprintf("%02x%062d", i%256, i) }

func ident(i int) Identity {
	return Identity{UserID: int64(i), Login: fmt.Sprintf("u%d", i), Role: RoleMarketer, TokenHash: h(i)}
}

func TestSessionCacheTTL(t *testing.T) {
	clk := clock.NewFake(t0)
	c := newSessionCache(clk, 5*time.Second, 100)
	c.put(h(1), ident(1), t0.Add(time.Hour), c.generation())

	if _, ok := c.get(h(1)); !ok {
		t.Fatal("запись должна быть в кэше")
	}
	clk.Advance(4 * time.Second)
	if _, ok := c.get(h(1)); !ok {
		t.Fatal("через 4 с запись ещё жива")
	}
	clk.Advance(time.Second)
	if _, ok := c.get(h(1)); ok {
		t.Fatal("через 5 с запись должна истечь")
	}
}

func TestSessionCacheNotLongerThanSession(t *testing.T) {
	clk := clock.NewFake(t0)
	c := newSessionCache(clk, time.Minute, 100)
	c.put(h(1), ident(1), t0.Add(2*time.Second), c.generation())
	clk.Advance(2 * time.Second)
	if _, ok := c.get(h(1)); ok {
		t.Fatal("запись не должна жить дольше expires_at сессии")
	}
	// Уже истёкшую сессию не кладём вовсе.
	fresh := newSessionCache(clk, time.Minute, 100)
	fresh.put(h(2), ident(2), t0, fresh.generation())
	if fresh.size() != 0 {
		t.Fatalf("в кэше %d записей, ждали 0", fresh.size())
	}
}

func TestSessionCacheEviction(t *testing.T) {
	clk := clock.NewFake(t0)
	// 16 шардов по 2 записи.
	c := newSessionCache(clk, time.Minute, 2*sessionCacheShards)
	// Три ключа в одном шарде (одинаковые первые байты хэша).
	k := func(i int) string { return fmt.Sprintf("00%062d", i) }
	c.put(k(1), ident(1), t0.Add(time.Hour), c.generation())
	clk.Advance(time.Second)
	c.put(k(2), ident(2), t0.Add(time.Hour), c.generation())
	clk.Advance(time.Second)
	c.put(k(3), ident(3), t0.Add(time.Hour), c.generation())

	if _, ok := c.get(k(1)); ok {
		t.Fatal("самая старая запись должна быть вытеснена")
	}
	for _, i := range []int{2, 3} {
		if _, ok := c.get(k(i)); !ok {
			t.Fatalf("запись %d должна остаться", i)
		}
	}

	// Истёкшие вытесняются раньше живых.
	clk.Advance(time.Minute - time.Second) // k2 (положена на 1-й с) истекла, k3 (на 2-й с) жива
	c.put(k(4), ident(4), t0.Add(time.Hour), c.generation())
	if _, ok := c.get(k(4)); !ok {
		t.Fatal("новая запись должна лечь в кэш")
	}
	if _, ok := c.get(k(3)); !ok {
		t.Fatal("живая запись не должна вытесняться, пока есть истёкшая")
	}
}

func TestSessionCacheSizeBound(t *testing.T) {
	c := newSessionCache(clock.NewFake(t0), time.Minute, 160)
	for i := 0; i < 5000; i++ {
		c.put(h(i), ident(i), t0.Add(time.Hour), c.generation())
	}
	if n := c.size(); n > 160 {
		t.Fatalf("в кэше %d записей, предел 160", n)
	}
}

func TestSessionCacheDisabled(t *testing.T) {
	if c := newSessionCache(clock.NewFake(t0), 0, 100); c != nil {
		t.Fatal("ttl = 0 выключает кэш")
	}
	var c *sessionCache
	c.put(h(1), ident(1), t0.Add(time.Hour), c.generation())
	if _, ok := c.get(h(1)); ok {
		t.Fatal("выключенный кэш ничего не хранит")
	}
	c.delete(h(1))
	c.deleteUser(1)
}

// newTestService — сервис с подменённой загрузкой сессии, без БД.
func newTestService(clk clock.Clock, ttl time.Duration, load func(string) (sessionRecord, error), calls *atomic.Int64) *service {
	s := &service{clk: clk, cache: newSessionCache(clk, ttl, 100)}
	s.load = func(_ context.Context, hash string) (sessionRecord, error) {
		calls.Add(1)
		return load(hash)
	}
	return s
}

func okLoad(role string, expires time.Time) func(string) (sessionRecord, error) {
	return func(hash string) (sessionRecord, error) {
		return sessionRecord{Identity: Identity{UserID: 1, Login: "u", Role: role, TokenHash: hash}, ExpiresAt: expires}, nil
	}
}

func TestAuthenticateSingleflight(t *testing.T) {
	var calls atomic.Int64
	release := make(chan struct{})
	s := newTestService(clock.NewFake(t0), 5*time.Second, func(hash string) (sessionRecord, error) {
		<-release
		return okLoad(RoleMarketer, t0.Add(time.Hour))(hash)
	}, &calls)

	const n = 100
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			id, err := s.authenticate(context.Background(), "tok")
			if err == nil && id.Role != RoleMarketer {
				err = errors.New("не та роль")
			}
			errs <- err
		}()
	}
	// Дать горутинам дойти до singleflight, затем отпустить единственный запрос.
	time.Sleep(100 * time.Millisecond)
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("запросов к БД %d, ждали 1", got)
	}
}

func TestAuthenticateCases(t *testing.T) {
	loadErr := errors.New("база недоступна")
	tests := []struct {
		name  string
		steps func(t *testing.T, s *service, clk *clock.Fake, calls *atomic.Int64)
	}{
		{"повтор идёт из кэша", func(t *testing.T, s *service, _ *clock.Fake, calls *atomic.Int64) {
			mustAuth(t, s, "tok")
			mustAuth(t, s, "tok")
			wantCalls(t, calls, 1)
		}},
		{"после TTL снова запрос в БД", func(t *testing.T, s *service, clk *clock.Fake, calls *atomic.Int64) {
			mustAuth(t, s, "tok")
			clk.Advance(6 * time.Second)
			mustAuth(t, s, "tok")
			wantCalls(t, calls, 2)
		}},
		{"выход убирает запись сразу", func(t *testing.T, s *service, _ *clock.Fake, calls *atomic.Int64) {
			id := mustAuth(t, s, "tok")
			s.cache.delete(id.TokenHash) // logout без БД: в нём DeleteSession + cache.delete
			mustAuth(t, s, "tok")
			wantCalls(t, calls, 2)
		}},
		{"нет сессии не кэшируется", func(t *testing.T, s *service, _ *clock.Fake, calls *atomic.Int64) {
			for i := 0; i < 3; i++ {
				if _, err := s.authenticate(context.Background(), "unknown"); !errors.Is(err, ErrNoSession) {
					t.Fatalf("ждали ErrNoSession, получили %v", err)
				}
			}
			wantCalls(t, calls, 3)
			if s.cache.size() != 0 {
				t.Fatal("отрицательный ответ попал в кэш")
			}
		}},
		{"ошибка БД не кэшируется", func(t *testing.T, s *service, _ *clock.Fake, calls *atomic.Int64) {
			if _, err := s.authenticate(context.Background(), "boom"); !errors.Is(err, loadErr) {
				t.Fatalf("ждали ошибку БД, получили %v", err)
			}
			if s.cache.size() != 0 {
				t.Fatal("ошибка попала в кэш")
			}
		}},
		{"пустой токен", func(t *testing.T, s *service, _ *clock.Fake, calls *atomic.Int64) {
			if _, err := s.authenticate(context.Background(), ""); !errors.Is(err, ErrNoSession) {
				t.Fatalf("получили %v", err)
			}
			wantCalls(t, calls, 0)
		}},
		{"смена роли сбрасывает сессии пользователя", func(t *testing.T, s *service, _ *clock.Fake, calls *atomic.Int64) {
			mustAuth(t, s, "tok")
			s.cache.deleteUser(1)
			mustAuth(t, s, "tok")
			wantCalls(t, calls, 2)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clk := clock.NewFake(t0)
			var calls atomic.Int64
			s := newTestService(clk, 5*time.Second, func(hash string) (sessionRecord, error) {
				switch hash {
				case hashToken("unknown"):
					return sessionRecord{}, ErrNoSession
				case hashToken("boom"):
					return sessionRecord{}, loadErr
				}
				return okLoad(RoleMarketer, t0.Add(time.Hour))(hash)
			}, &calls)
			tt.steps(t, s, clk, &calls)
		})
	}
}

func TestAuthenticateStaleLoadNotCachedAfterLogout(t *testing.T) {
	var calls atomic.Int64
	started, release := make(chan struct{}), make(chan struct{})
	s := newTestService(clock.NewFake(t0), 5*time.Second, func(hash string) (sessionRecord, error) {
		close(started)
		<-release
		return okLoad(RoleMarketer, t0.Add(time.Hour))(hash)
	}, &calls)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = s.authenticate(context.Background(), "tok")
	}()
	<-started
	s.cache.delete(hashToken("tok")) // выход, пока загрузка в полёте
	close(release)
	<-done
	if s.cache.size() != 0 {
		t.Fatal("загрузка, начатая до выхода, положила сессию в кэш")
	}
}

func TestAuthenticateLeaderCancelDoesNotFailOthers(t *testing.T) {
	var calls atomic.Int64
	s := newTestService(clock.NewFake(t0), 5*time.Second, okLoad(RoleMarketer, t0.Add(time.Hour)), &calls)
	s.load = func(ctx context.Context, hash string) (sessionRecord, error) {
		calls.Add(1)
		select {
		case <-ctx.Done():
			return sessionRecord{}, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
		return okLoad(RoleMarketer, t0.Add(time.Hour))(hash)
	}
	ctx, cancel := context.WithCancel(context.Background())
	res := make(chan error, 2)
	go func() { _, err := s.authenticate(ctx, "tok"); res <- err }()
	time.Sleep(10 * time.Millisecond)
	go func() { _, err := s.authenticate(context.Background(), "tok"); res <- err }()
	time.Sleep(10 * time.Millisecond)
	cancel()
	for i := 0; i < 2; i++ {
		if err := <-res; err != nil {
			t.Fatalf("запрос %d: %v", i, err)
		}
	}
}

func TestSessionCacheMetrics(t *testing.T) {
	clk := clock.NewFake(t0)
	var calls atomic.Int64
	s := newTestService(clk, 5*time.Second, okLoad(RoleMarketer, t0.Add(time.Hour)), &calls)
	mustAuth(t, s, "tok") // промах
	mustAuth(t, s, "tok") // попадание
	mustAuth(t, s, "tok") // попадание

	api := &AuthAPI{svc: s}
	want := `
# HELP churn_session_cache_total Проверки сессии по кэшу процесса (штуки); result: hit — без запроса в БД, miss — с запросом.
# TYPE churn_session_cache_total counter
churn_session_cache_total{result="hit"} 2
churn_session_cache_total{result="miss"} 1
`
	if err := testutil.CollectAndCompare(api.Collector(), strings.NewReader(want)); err != nil {
		t.Fatal(err)
	}
}

func TestSessionCacheTTLFromEnv(t *testing.T) {
	for env, want := range map[string]time.Duration{"": 5 * time.Second, "30s": 30 * time.Second, "0": 0, "-1s": 5 * time.Second, "abc": 5 * time.Second} {
		t.Setenv("SESSION_CACHE_TTL", env)
		if got := sessionCacheTTLFromEnv(); got != want {
			t.Errorf("SESSION_CACHE_TTL=%q: %v, ждали %v", env, got, want)
		}
	}
}

func mustAuth(t *testing.T, s *service, token string) Identity {
	t.Helper()
	id, err := s.authenticate(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func wantCalls(t *testing.T, calls *atomic.Int64, want int64) {
	t.Helper()
	if got := calls.Load(); got != want {
		t.Fatalf("запросов к БД %d, ждали %d", got, want)
	}
}

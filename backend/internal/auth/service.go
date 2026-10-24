package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/singleflight"

	"example.com/app/internal/auth/db"
	"example.com/app/internal/platform/clock"
)

const (
	sessionTTL    = 12 * time.Hour
	maxFailures   = 5
	failureWindow = 5 * time.Minute
)

// Ошибки входа. Наружу не говорят, что именно неверно.
var (
	ErrBadCredentials = errors.New("auth: неверный логин или пароль")
	ErrNoSession      = errors.New("auth: нет сессии")
)

// Identity — пользователь и сессия, найденные по токену.
type Identity struct {
	UserID    int64
	Login     string
	Role      string
	CSRFToken string
	TokenHash string
}

// Session — результат входа: токены отдаются клиенту один раз.
type Session struct {
	Identity
	Token     string
	ExpiresAt time.Time
}

type service struct {
	pool *pgxpool.Pool
	q    *db.Queries
	clk  clock.Clock
	cfg  Config

	// cache и flight: проверка сессии без похода в БД; cache == nil, если кэш выключен.
	cache  *sessionCache
	flight singleflight.Group
	// load читает живую сессию из БД; в тестах подменяется.
	load func(ctx context.Context, tokenHash string) (sessionRecord, error)

	mu       sync.Mutex
	failures map[string][]time.Time
	// dummyHash выравнивает время ответа для несуществующего логина.
	dummyHash string
}

func newService(pool *pgxpool.Pool, clk clock.Clock, cfg Config) *service {
	s := &service{pool: pool, clk: clk, cfg: cfg, failures: map[string][]time.Time{}}
	if pool != nil {
		s.q = db.New(pool)
	}
	s.cache = newSessionCache(clk, cfg.SessionCacheTTL, sessionCacheMaxSize)
	if pool != nil {
		s.load = s.loadSession
	}
	s.dummyHash, _ = HashPassword("dummy-password")
	return s
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: случайный токен: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// ensureUsers создаёт администратора и демо-учётки из настроек. Учётка без пароля не
// создаётся; изменившийся пароль или роль перезаписываются.
func (s *service) ensureUsers(ctx context.Context) error {
	type account struct{ login, password, role string }
	var accounts []account
	if s.cfg.AdminLogin != "" && s.cfg.AdminPassword != "" {
		accounts = append(accounts, account{s.cfg.AdminLogin, s.cfg.AdminPassword, RoleAdmin})
	}
	for _, role := range []string{RoleMarketer, RoleAnalyst, RoleJury} {
		if p := s.cfg.DemoPasswords[role]; p != "" {
			accounts = append(accounts, account{role, p, role})
		}
	}
	for _, a := range accounts {
		existing, err := s.q.GetUserByLogin(ctx, a.login)
		if err == nil && existing.Role == a.role && VerifyPassword(a.password, existing.PasswordHash) {
			continue
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("auth: учётка %s: %w", a.login, err)
		}
		hash, err := HashPassword(a.password)
		if err != nil {
			return err
		}
		if _, err := s.q.UpsertUser(ctx, db.UpsertUserParams{Login: a.login, PasswordHash: hash, Role: a.role}); err != nil {
			return fmt.Errorf("auth: учётка %s: %w", a.login, err)
		}
	}
	return nil
}

// reserveAttempt атомарно проверяет лимит и занимает попытку: параллельные запросы не могут
// пройти проверку, пока argon2 считает хеш. Если лимит исчерпан, возвращает время до
// освобождения окна. Успешный вход снимает занятые попытки через clearFailures.
func (s *service) reserveAttempt(key string) (time.Duration, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clk.Now()
	recent := s.failures[key][:0]
	for _, t := range s.failures[key] {
		if now.Sub(t) < failureWindow {
			recent = append(recent, t)
		}
	}
	if len(recent) >= maxFailures {
		s.failures[key] = recent
		return recent[0].Add(failureWindow).Sub(now), true
	}
	s.failures[key] = append(recent, now)
	return 0, false
}

func (s *service) clearFailures(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.failures, key)
}

// login проверяет пароль и заводит сессию.
func (s *service) login(ctx context.Context, login, password string) (Session, error) {
	if s.q == nil {
		return Session{}, ErrBadCredentials
	}
	user, err := s.q.GetUserByLogin(ctx, login)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		VerifyPassword(password, s.dummyHash)
		return Session{}, ErrBadCredentials
	case err != nil:
		return Session{}, fmt.Errorf("auth: пользователь: %w", err)
	}
	if !VerifyPassword(password, user.PasswordHash) {
		return Session{}, ErrBadCredentials
	}
	return s.issueSession(ctx, user.ID, user.Login, user.Role)
}

// issueSession заводит сессию для пользователя; общая для входа по паролю и через SSO.
func (s *service) issueSession(ctx context.Context, userID int64, login, role string) (Session, error) {
	token, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	csrf, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	now := s.clk.Now()
	expires := now.Add(sessionTTL)
	if err := s.q.DeleteExpiredSessions(ctx, now); err != nil {
		return Session{}, fmt.Errorf("auth: очистка сессий: %w", err)
	}
	if err := s.q.CreateSession(ctx, db.CreateSessionParams{
		TokenHash: hashToken(token), UserID: userID, CsrfToken: csrf, ExpiresAt: expires,
	}); err != nil {
		return Session{}, fmt.Errorf("auth: сессия: %w", err)
	}
	return Session{
		Identity:  Identity{UserID: userID, Login: login, Role: role, CSRFToken: csrf, TokenHash: hashToken(token)},
		Token:     token,
		ExpiresAt: expires,
	}, nil
}

// sessionRecord — живая сессия из БД вместе с её сроком.
type sessionRecord struct {
	Identity
	ExpiresAt time.Time
}

func (s *service) loadSession(ctx context.Context, tokenHash string) (sessionRecord, error) {
	row, err := s.q.GetSession(ctx, db.GetSessionParams{TokenHash: tokenHash, ExpiresAt: s.clk.Now()})
	if errors.Is(err, pgx.ErrNoRows) {
		return sessionRecord{}, ErrNoSession
	}
	if err != nil {
		return sessionRecord{}, fmt.Errorf("auth: сессия: %w", err)
	}
	return sessionRecord{
		Identity:  Identity{UserID: row.UserID, Login: row.Login, Role: row.Role, CSRFToken: row.CsrfToken, TokenHash: row.TokenHash},
		ExpiresAt: row.ExpiresAt,
	}, nil
}

// authenticate находит живую сессию по токену: сначала в кэше процесса, при промахе в БД.
// Одновременные промахи по одному токену делят один запрос.
func (s *service) authenticate(ctx context.Context, token string) (Identity, error) {
	if s.load == nil || token == "" {
		return Identity{}, ErrNoSession
	}
	hash := hashToken(token)
	if id, ok := s.cache.get(hash); ok {
		return id, nil
	}
	v, err, _ := s.flight.Do(hash, func() (any, error) {
		gen := s.cache.generation()
		// Запрос общий для всех ожидающих: отмена одного клиента не должна ронять остальных.
		rec, err := s.load(context.WithoutCancel(ctx), hash)
		if err != nil {
			return nil, err // «нет сессии» не кэшируем
		}
		s.cache.put(hash, rec.Identity, rec.ExpiresAt, gen)
		return rec.Identity, nil
	})
	if err != nil {
		return Identity{}, err
	}
	return v.(Identity), nil
}

func (s *service) logout(ctx context.Context, tokenHash string) error {
	if s.q == nil {
		return nil
	}
	err := s.q.DeleteSession(ctx, tokenHash)
	s.cache.delete(tokenHash)
	return err
}

// ssoLogin находит или заводит SSO-учётку с ролью из провайдера и открывает сессию.
func (s *service) ssoLogin(ctx context.Context, login, role string) (Session, error) {
	if s.q == nil {
		return Session{}, ErrNoSession
	}
	u, err := s.q.UpsertSSOUser(ctx, db.UpsertSSOUserParams{Login: login, Role: role})
	if err != nil {
		return Session{}, fmt.Errorf("auth: SSO-учётка %s: %w", login, err)
	}
	// Роль могла измениться: старая не должна жить в кэше других сессий пользователя.
	s.cache.deleteUser(u.ID)
	return s.issueSession(ctx, u.ID, u.Login, u.Role)
}

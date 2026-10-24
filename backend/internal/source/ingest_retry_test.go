package source

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"
)

func fastRetry(t *testing.T) {
	t.Helper()
	p, e := ingestRetryPauses, ingestPingEvery
	ingestRetryPauses = []time.Duration{time.Millisecond, time.Millisecond}
	ingestPingEvery = time.Millisecond
	t.Cleanup(func() { ingestRetryPauses, ingestPingEvery = p, e })
}

// База пропала на много попыток: пачка ждёт ping и пишется целиком, ошибки наружу нет.
func TestRetryCommitWaitsForDB(t *testing.T) {
	fastRetry(t)
	down := 4 // больше, чем пауз (2): доходит до ожидания ping
	pings := 0
	calls := 0
	commit := func() (int64, map[int]error, error) {
		calls++
		if down > 0 {
			down--
			return 0, nil, syscall.ECONNREFUSED
		}
		return 7, nil, nil
	}
	ping := func(context.Context) error {
		pings++
		if pings < 3 {
			return syscall.ECONNREFUSED
		}
		return nil
	}
	v, _, err := retryCommit(context.Background(), 3, commit, ping)
	if err != nil || v != 7 {
		t.Fatalf("версия %d, ошибка %v", v, err)
	}
	if calls != 5 || pings != 4 {
		t.Fatalf("попыток %d (ждали 5), ping %d (ждали 4)", calls, pings)
	}
}

// Ошибка запроса (не отказ базы): один повтор, затем отказ.
func TestRetryCommitQueryErrorTwice(t *testing.T) {
	fastRetry(t)
	boom := errors.New("синтаксис")
	calls := 0
	_, _, err := retryCommit(context.Background(), 1, func() (int64, map[int]error, error) {
		calls++
		return 0, nil, boom
	}, func(context.Context) error { return nil })
	if !errors.Is(err, boom) || calls != 2 {
		t.Fatalf("ошибка %v, попыток %d", err, calls)
	}
}

// Остановка процесса прерывает ожидание базы.
func TestRetryCommitStopsOnCancel(t *testing.T) {
	fastRetry(t)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(20 * time.Millisecond); cancel() }()
	_, _, err := retryCommit(ctx, 1,
		func() (int64, map[int]error, error) { return 0, nil, syscall.ECONNREFUSED },
		func(context.Context) error { return syscall.ECONNREFUSED })
	if err == nil {
		t.Fatal("ждали ошибку отмены")
	}
}

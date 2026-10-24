package model

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestSensitivityCacheByVersions(t *testing.T) {
	var c sensitivityCache
	var calls atomic.Int32
	compute := func(snap, set int64) func() (Sensitivity, error) {
		return func() (Sensitivity, error) {
			calls.Add(1)
			return Sensitivity{SnapshotVersion: snap, SettingsVersion: set, Customers: 3}, nil
		}
	}
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := c.get(ctx, sensKey{1, 1}, compute(1, 1)); err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("тех же версий расчётов %d, ждали 1", calls.Load())
	}
	if _, err := c.get(ctx, sensKey{2, 1}, compute(2, 1)); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("новая версия снимка не пересчитана: %d", calls.Load())
	}
}

func TestSensitivityCacheSingleFlight(t *testing.T) {
	var c sensitivityCache
	var calls atomic.Int32
	release := make(chan struct{})
	compute := func() (Sensitivity, error) {
		calls.Add(1)
		<-release
		return Sensitivity{SnapshotVersion: 1, SettingsVersion: 1}, nil
	}
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { _, err := c.get(context.Background(), sensKey{1, 1}, compute); done <- err }()
	}
	time.Sleep(50 * time.Millisecond)
	close(release)
	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 {
		t.Fatalf("одновременных расчётов %d, ждали 1", calls.Load())
	}
}

func TestSensitivityCacheTimeoutKeepsCalculating(t *testing.T) {
	var c sensitivityCache
	release := make(chan struct{})
	slow := func() (Sensitivity, error) {
		<-release
		return Sensitivity{SnapshotVersion: 1, SettingsVersion: 1}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := c.get(ctx, sensKey{1, 1}, slow); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("ждали таймаут, получили %v", err)
	}
	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for {
		var calls int
		_, err := c.get(context.Background(), sensKey{1, 1}, func() (Sensitivity, error) { calls++; return Sensitivity{}, nil })
		if err == nil && calls == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("результат медленного расчёта не попал в кэш")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSensitivityCacheDoesNotStoreErrors(t *testing.T) {
	var c sensitivityCache
	boom := errors.New("boom")
	if _, err := c.get(context.Background(), sensKey{1, 1}, func() (Sensitivity, error) { return Sensitivity{}, boom }); !errors.Is(err, boom) {
		t.Fatalf("ждали ошибку, получили %v", err)
	}
	if _, err := c.get(context.Background(), sensKey{1, 1}, func() (Sensitivity, error) { return Sensitivity{SnapshotVersion: 1, SettingsVersion: 1}, nil }); err != nil {
		t.Fatalf("ошибка не должна кэшироваться: %v", err)
	}
}

//go:build integration

package app_test

import (
	"bytes"
	"context"
	"os"
	"runtime"
	"runtime/metrics"
	"sync"
	"testing"
	"time"

	"example.com/app/internal/apitest"
	"example.com/app/tools/snapgen"
)

// heapBytes — живые и ещё не собранные объекты кучи (дёшево, без остановки мира).
func heapBytes() uint64 {
	s := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	metrics.Read(s)
	return s[0].Value.Uint64()
}

// peakHeapDuring возвращает кучу до вызова f (после сборки мусора) и её пик за время f.
func peakHeapDuring(f func()) (before, peak uint64) {
	runtime.GC()
	before = heapBytes()
	peak = before
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if h := heapBytes(); h > peak {
				peak = h
			}
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
	}()
	f()
	close(stop)
	wg.Wait()
	return before, peak
}

// putGrowth грузит снимок на n аккаунтов и возвращает рост кучи за PUT (пик минус до) в МБ.
// Пересчёт не запущен, поэтому в кучу попадает только сам приём; тело снимка лежит в памяти
// теста и входит в «до».
func putGrowth(t *testing.T, accounts int) float64 {
	t.Helper()
	body, err := snapgen.Generate(snapgen.Options{Accounts: accounts, Seed: 11})
	if err != nil {
		t.Fatal(err)
	}
	r := newRig(t, false)
	loader := r.WithBearer(t, apitest.SourceToken)
	var took time.Duration
	before, peak := peakHeapDuring(func() {
		start := time.Now()
		res, err := loader.PutSnapshotWithBodyWithResponse(context.Background(), "application/json", bytes.NewReader(body))
		if err != nil || res.JSON200 == nil {
			t.Errorf("PUT: %v %d %s", err, res.StatusCode(), res.Body)
			return
		}
		took = time.Since(start)
		t.Logf("аккаунтов %d, услуг %d, платежей %d; тело %.1f МБ", res.JSON200.Accounts, res.JSON200.Services, res.JSON200.Transactions, float64(len(body))/(1<<20))
	})
	growth := float64(peak-before) / (1 << 20)
	t.Logf("PUT %v; куча до %.0f МБ, пик %.0f МБ, рост %.0f МБ", took.Round(time.Millisecond), float64(before)/(1<<20), float64(peak)/(1<<20), growth)
	return growth
}

// Пик памяти приёма не растёт вместе с размером снимка: на тройном объёме рост кучи меньше
// чем вдвое больше, чем на одинарном (при линейном росте было бы втрое). Граница записана здесь.
// 300k аккаунтов — только в прогоне с тегом load или LOAD_300K=1: это ~ 330 МБ тела.
func TestSnapshotIngestPeakMemory(t *testing.T) {
	small := putGrowth(t, 100000)
	if os.Getenv("LOAD_300K") != "1" {
		t.Skip("300k аккаунтов: задайте LOAD_300K=1 (≈ 1 ГБ памяти, ≈ 1 минута)")
	}
	large := putGrowth(t, 300000)
	if large > small*2 {
		t.Errorf("рост кучи на 300k (%.0f МБ) больше чем вдвое выше, чем на 100k (%.0f МБ): память растёт вместе со снимком", large, small)
	}
}

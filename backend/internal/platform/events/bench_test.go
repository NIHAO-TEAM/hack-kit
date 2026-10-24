package events_test

import (
	"fmt"
	"sync"
	"testing"

	"example.com/app/internal/platform/events"
)

// drain читает канал до закрытия, чтобы подписчики не копили буфер и не тормозили замер.
func drain(wg *sync.WaitGroup, ch <-chan events.Event) {
	defer wg.Done()
	for range ch {
	}
}

func benchPublish(b *testing.B, subs int) {
	bus := events.New()
	var wg sync.WaitGroup
	cancels := make([]func(), 0, subs)
	for i := 0; i < subs; i++ {
		ch, cancel := bus.Subscribe("t")
		cancels = append(cancels, cancel)
		wg.Add(1)
		go drain(&wg, ch)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bus.Publish("t", i)
	}
	b.StopTimer()
	for _, c := range cancels {
		c()
	}
	wg.Wait()
}

func BenchmarkPublish1Subs(b *testing.B)    { benchPublish(b, 1) }
func BenchmarkPublish100Subs(b *testing.B)  { benchPublish(b, 100) }
func BenchmarkPublish1000Subs(b *testing.B) { benchPublish(b, 1000) }

// BenchmarkPublishParallel — несколько издателей одновременно при 100 подписчиках.
func BenchmarkPublishParallel(b *testing.B) {
	bus := events.New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		ch, cancel := bus.Subscribe("t")
		defer cancel()
		wg.Add(1)
		go drain(&wg, ch)
	}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for i := 0; pb.Next(); i++ {
			bus.Publish("t", fmt.Sprint(i&7))
		}
	})
}

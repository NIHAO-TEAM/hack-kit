package app

import (
	"context"
	"errors"
	"log"
	"os"
	"time"

	"example.com/app/internal/platform/events"
	"example.com/app/internal/source"
)

// defaultPeerPoll — как часто реплика сверяет версию снимка в базе с тем, что знает сама (RECALC_PEER_POLL).
const defaultPeerPoll = 2 * time.Second

// peerPollFromEnv читает RECALC_PEER_POLL (time.ParseDuration). 0 — сверка выключена (одна реплика).
// Пусто или неверно — 2 с, неверное значение попадает в лог.
func peerPollFromEnv() time.Duration {
	v := os.Getenv("RECALC_PEER_POLL")
	if v == "" {
		return defaultPeerPoll
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		log.Printf("app: RECALC_PEER_POLL=%q не длительность вида 2s, беру %s", v, defaultPeerPoll)
		return defaultPeerPoll
	}
	return d
}

// watchPeers замечает снимок, записанный другой репликой: шина событий живёт в процессе, и реплика,
// которой загрузку не прислали, о ней иначе не узнает (до перезапуска отвечала бы 503 «расчёт ещё не готов»
// либо считала бы по старым данным). Раз в r.peerPoll читает версию снимка из базы; если она новее той,
// что реплика видела сама (событие своей шины или результат пересчёта) и так два опроса подряд, объявляет
// source.changed без клиентов — пересчёт полный. Два опроса подряд нужны, чтобы не принять за чужую
// запись свою, чьё событие ещё не дошло до шины.
func (r *Recalc) watchPeers(ctx context.Context, bus *events.Bus) <-chan struct{} {
	done := make(chan struct{})
	if r.peerPoll <= 0 {
		close(done)
		return done
	}
	ch, cancel := bus.Subscribe(source.TopicChanged)
	go func() {
		defer close(done)
		defer cancel()
		var seen, suspect int64 // seen — новейшая версия, которую реплика знает сама
		t := time.NewTicker(r.peerPoll)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case ev := <-ch:
				if c, ok := ev.Payload.(source.Change); ok && c.Version > seen {
					seen = c.Version
				}
			case <-t.C:
				if c := r.Engine.Latest(); c != nil {
					if c.DataVersion > seen {
						seen = c.DataVersion
					}
				} else if !errors.Is(r.Engine.LastError(), source.ErrNoSnapshot) {
					continue // первый пересчёт ещё идёт: он сам прочтёт свежий снимок
				}
				v, err := r.reader.Version(ctx)
				if err != nil {
					if !errors.Is(err, source.ErrNoSnapshot) && ctx.Err() == nil {
						log.Printf("app: сверка версии снимка: %v", err)
					}
					continue
				}
				switch {
				case v <= seen:
					suspect = 0
				case v != suspect:
					suspect = v // первый опрос с новой версией: ждём подтверждения
				default:
					log.Printf("app: снимок версии %d записан другой репликой (знаю %d): пересчёт", v, seen)
					seen, suspect = v, 0
					bus.Publish(source.TopicChanged, source.Change{Version: v})
				}
			}
		}
	}()
	return done
}

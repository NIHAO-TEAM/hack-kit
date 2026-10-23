// Package events — шина событий внутри процесса. Её создаёт cmd/api и передаёт модулям.
package events

import "sync"

// Event — событие шины.
type Event struct {
	Topic   string
	Payload any
}

// Bus рассылает события подписчикам темы. Медленный подписчик не блокирует издателя:
// у каждого буфер, при переполнении самое старое событие теряется.
type Bus struct {
	mu   sync.Mutex
	subs map[string]map[chan Event]struct{}
}

// New создаёт пустую шину.
func New() *Bus {
	return &Bus{subs: map[string]map[chan Event]struct{}{}}
}

const buffer = 16

// Publish отправляет событие всем подписчикам темы.
func (b *Bus) Publish(topic string, payload any) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ev := Event{Topic: topic, Payload: payload}
	for ch := range b.subs[topic] {
		select {
		case ch <- ev:
		default:
			select { // буфер полон: выбрасываем самое старое
			case <-ch:
			default:
			}
			select {
			case ch <- ev:
			default:
			}
		}
	}
}

// Subscribe подписывает на тему. cancel отписывает и закрывает канал.
func (b *Bus) Subscribe(topic string) (events <-chan Event, cancel func()) {
	ch := make(chan Event, buffer)
	b.mu.Lock()
	if b.subs[topic] == nil {
		b.subs[topic] = map[chan Event]struct{}{}
	}
	b.subs[topic][ch] = struct{}{}
	b.mu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subs[topic], ch)
			b.mu.Unlock()
			close(ch)
		})
	}
}

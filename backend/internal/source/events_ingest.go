package source

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"example.com/app/internal/platform/api/gen"
)

// Пределы запроса POST /source/events.
const (
	eventsMaxCount  = 5000
	eventsMaxBytes  = 2 << 20
	eventsMaxLine   = 64 << 10
	eventsMaxIDLen  = 128
	eventsQueueWait = time.Second // сколько запрос ждёт место в очереди писателя
	eventsIDTTL     = 24 * time.Hour
	eventsCleanEach = time.Hour
)

// Ошибки разбора тела запроса приёма.
var (
	errEventsTooMany = fmt.Errorf("в запросе больше %d событий", eventsMaxCount)
	errEventsBusy    = errors.New("очередь записи событий полна")
)

// eventsBodyError — тело не разобрано: запрос отклоняется целиком (400).
type eventsBodyError struct{ error }

var eventsKinds = map[string]bool{
	EventTopUp: true, EventAutoprolongOn: true, EventAutoprolongOff: true, EventServiceStop: true,
	EventServiceResume: true, EventRenewalFailed: true, EventProlongWait: true,
}

var (
	ingestAPIEvents = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "churn_ingest_api_events_total",
		Help: "События POST /source/events; result: accepted — в очереди записи, duplicate — повтор eventId, rejected — не прошло проверку, applied/failed — итог записи (штуки).",
	}, []string{"result"})
	ingestAPIShed = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "churn_ingest_api_shed_total",
		Help: "Запросы POST /source/events, отклонённые из-за полной очереди записи (штуки).",
	})
)

// rawEvent — событие запроса до проверки; line — номер строки NDJSON или позиция в массиве, с 1.
type rawEvent struct {
	line int
	data []byte
}

// readEvents разбирает тело запроса: JSON-массив, если первый значащий символ «[», иначе NDJSON.
// Пустые строки NDJSON пропускаются. Больше eventsMaxCount событий — errEventsTooMany.
func readEvents(body io.Reader) ([]rawEvent, error) {
	br := bufio.NewReaderSize(body, eventsMaxLine)
	for {
		c, err := br.ReadByte()
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			continue
		}
		_ = br.UnreadByte()
		if c == '[' {
			return readEventsArray(br)
		}
		return readEventsLines(br)
	}
}

func readEventsArray(r io.Reader) ([]rawEvent, error) {
	dec := json.NewDecoder(r)
	if _, err := dec.Token(); err != nil {
		return nil, wrapBody(err)
	}
	var out []rawEvent
	for dec.More() {
		if len(out) == eventsMaxCount {
			return nil, errEventsTooMany
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, wrapBody(err)
		}
		out = append(out, rawEvent{line: len(out) + 1, data: raw})
	}
	if _, err := dec.Token(); err != nil {
		return nil, wrapBody(err)
	}
	return out, nil
}

func readEventsLines(r io.Reader) ([]rawEvent, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, eventsMaxLine), eventsMaxLine)
	var out []rawEvent
	for line := 1; sc.Scan(); line++ {
		text := bytes.TrimSpace(sc.Bytes())
		if len(text) == 0 {
			continue
		}
		if len(out) == eventsMaxCount {
			return nil, errEventsTooMany
		}
		out = append(out, rawEvent{line: line, data: append([]byte(nil), text...)})
	}
	if err := sc.Err(); err != nil {
		if errors.Is(err, bufio.ErrTooLong) {
			return nil, eventsBodyError{fmt.Errorf("строка длиннее %d КБ", eventsMaxLine>>10)}
		}
		return nil, err
	}
	return out, nil
}

// wrapBody помечает синтаксическую ошибку JSON как ошибку тела; ошибки чтения (лимит тела) проходят как есть.
func wrapBody(err error) error {
	var syn *json.SyntaxError
	var typ *json.UnmarshalTypeError
	if errors.As(err, &syn) || errors.As(err, &typ) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return eventsBodyError{err}
	}
	return err
}

// parsedEvent — событие, прошедшее проверку.
type parsedEvent struct {
	line int
	id   string
	ev   StreamEvent
}

// parseEvent проверяет одно событие запроса без чтения базы: формат, обязательные поля по виду
// события и значения теми же правилами, что у PATCH (ingestState.check).
func parseEvent(raw rawEvent, st *ingestState) (parsedEvent, error) {
	var w gen.IngestEvent
	if err := json.Unmarshal(raw.data, &w); err != nil {
		return parsedEvent{}, fmt.Errorf("не разобрано как событие: %v", err)
	}
	id := strings.TrimSpace(w.EventId)
	switch {
	case id == "":
		return parsedEvent{}, errors.New("eventId обязателен")
	case len(id) > eventsMaxIDLen:
		return parsedEvent{}, fmt.Errorf("eventId длиннее %d знаков", eventsMaxIDLen)
	case !eventsKinds[w.Kind]:
		return parsedEvent{}, fmt.Errorf("неизвестный вид события %q", w.Kind)
	}
	ev := StreamEvent{Kind: w.Kind, Balance: w.Balance, Status: w.Status, AutoProlong: w.AutoProlong, ExpiresAt: w.ExpiresAt}
	if w.AccountId != nil {
		ev.AccountID = *w.AccountId
	}
	if w.ServiceId != nil {
		ev.ServiceID = *w.ServiceId
	}
	if ev.Balance == nil && ev.Status == nil && ev.AutoProlong == nil && ev.ExpiresAt == nil {
		return parsedEvent{}, errors.New("событие ничего не меняет: нужен balance, status, autoProlong или expiresAt")
	}
	if ev.Balance != nil && ev.AccountID <= 0 {
		return parsedEvent{}, errors.New("для balance нужен accountId")
	}
	if (ev.Status != nil || ev.AutoProlong != nil || ev.ExpiresAt != nil) && ev.ServiceID <= 0 {
		return parsedEvent{}, errors.New("для status, autoProlong и expiresAt нужен serviceId")
	}
	switch ev.Kind {
	case EventTopUp:
		if ev.Balance == nil {
			return parsedEvent{}, errors.New("top_up: нужен balance")
		}
	case EventAutoprolongOn, EventAutoprolongOff:
		if ev.AutoProlong == nil {
			return parsedEvent{}, fmt.Errorf("%s: нужен autoProlong", ev.Kind)
		}
	default:
		if ev.Status == nil {
			return parsedEvent{}, fmt.Errorf("%s: нужен status", ev.Kind)
		}
	}
	if err := st.check(ev); err != nil {
		return parsedEvent{}, err
	}
	return parsedEvent{line: raw.line, id: id, ev: ev}, nil
}

// offerAll ставит все события в очередь писателя одним действием: ждёт, пока в очереди хватит места
// на все, не дольше wait; не хватило — не ставит ничего. Ставят только запросы приёма, а писатель
// лишь разбирает очередь, поэтому после проверки места постановка не блокируется.
func (g *eventWriter) offerAll(ctx context.Context, evs []StreamEvent, wait time.Duration) bool {
	if len(evs) > cap(g.in) {
		return false
	}
	g.offerMu.Lock()
	defer g.offerMu.Unlock()
	deadline := time.NewTimer(wait)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for cap(g.in)-len(g.in) < len(evs) {
		select {
		case <-deadline.C:
			return false
		case <-ctx.Done():
			return false
		case <-tick.C:
		}
	}
	at := g.now()
	for _, ev := range evs {
		g.in <- ingestItem{ev: ev, at: at}
	}
	return true
}

// eventIDStore хранит окно принятых eventId.
type eventIDStore interface {
	// claim занимает ещё не виденные id и возвращает их; транзакция держит занятые id, пока её не закроют:
	// откат освобождает их, чтобы повтор запроса после 503 не считался дублем.
	claim(ctx context.Context, ids []string, now time.Time) (fresh []string, tx idTx, err error)
	// clean убирает id, принятые раньше before.
	clean(ctx context.Context, before time.Time) error
}

type idTx interface {
	Commit(context.Context) error
	Rollback(context.Context) error
}

// eventIntake — приём событий извне: проверка, идемпотентность, постановка в очередь писателя.
type eventIntake struct {
	w       *eventWriter
	ids     eventIDStore
	version func(ctx context.Context) (int64, bool, error) // текущая версия данных; false — снимка ещё нет
	now     func() time.Time
	wait    time.Duration

	lastClean atomic.Int64 // unix-секунды последней чистки окна id
}

// ingestAPIQueueSize — очередь писателя приёма: вмещает самый большой запрос целиком.
const ingestAPIQueueSize = 2 * eventsMaxCount

// ingestOutcome — итог запроса.
type ingestOutcome struct {
	accepted, duplicate int
	rejected            []gen.IngestRejected
	version             int64
}

// process принимает запрос целиком. Ошибки: eventsBodyError — тело не разобрано, errEventsTooMany и
// ошибка лимита тела — слишком большой запрос, errEventsBusy — очередь полна (ничего не принято),
// errNoSnapshot — данных ещё нет, иное — база.
func (in *eventIntake) process(ctx context.Context, body io.Reader) (ingestOutcome, error) {
	raws, err := readEvents(body)
	if err != nil {
		return ingestOutcome{}, err
	}
	out := ingestOutcome{rejected: []gen.IngestRejected{}}
	var ok []parsedEvent
	seen := make(map[string]struct{}, len(raws))
	for _, raw := range raws {
		p, err := parseEvent(raw, in.w.state)
		if err != nil {
			out.rejected = append(out.rejected, gen.IngestRejected{Line: raw.line, Error: err.Error()})
			continue
		}
		if _, dup := seen[p.id]; dup {
			out.duplicate++
			continue
		}
		seen[p.id] = struct{}{}
		ok = append(ok, p)
	}
	if _, has, err := in.version(ctx); err != nil {
		return ingestOutcome{}, err
	} else if !has {
		return ingestOutcome{}, errNoSnapshot
	}
	if len(ok) > 0 {
		ids := make([]string, len(ok))
		for i, p := range ok {
			ids[i] = p.id
		}
		fresh, tx, err := in.ids.claim(ctx, ids, in.now())
		if err != nil {
			return ingestOutcome{}, err
		}
		isFresh := make(map[string]struct{}, len(fresh))
		for _, id := range fresh {
			isFresh[id] = struct{}{}
		}
		evs := make([]StreamEvent, 0, len(fresh))
		for _, p := range ok {
			if _, f := isFresh[p.id]; f {
				evs = append(evs, p.ev)
			}
		}
		out.duplicate += len(ok) - len(evs)
		if len(evs) > 0 && !in.w.offerAll(ctx, evs, in.wait) {
			_ = tx.Rollback(context.WithoutCancel(ctx))
			ingestAPIShed.Inc()
			return ingestOutcome{}, errEventsBusy
		}
		if err := tx.Commit(ctx); err != nil {
			return ingestOutcome{}, err
		}
		out.accepted = len(evs)
		in.cleanOld(ctx)
	}
	version, _, err := in.version(ctx)
	if err != nil {
		return ingestOutcome{}, err
	}
	out.version = version
	ingestAPIEvents.WithLabelValues("accepted").Add(float64(out.accepted))
	ingestAPIEvents.WithLabelValues("duplicate").Add(float64(out.duplicate))
	ingestAPIEvents.WithLabelValues("rejected").Add(float64(len(out.rejected)))
	return out, nil
}

// cleanOld раз в час убирает из окна id старше суток; сбой чистки приём не ломает.
func (in *eventIntake) cleanOld(ctx context.Context) {
	now := in.now()
	last := in.lastClean.Load()
	if now.Unix()-last < int64(eventsCleanEach/time.Second) || !in.lastClean.CompareAndSwap(last, now.Unix()) {
		return
	}
	_ = in.ids.clean(ctx, now.Add(-eventsIDTTL))
}

var errNoSnapshot = errors.New("снимка данных ещё нет")

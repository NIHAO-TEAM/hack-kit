package source

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func eventsBody(prefix string, n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "{\"eventId\":\"%s%d\",\"kind\":\"top_up\",\"accountId\":1,\"balance\":%d}\n", prefix, i, i)
	}
	return b.String()
}

// Писатель стоит, очередь на 10 событий. Запрос на 8 помещается, следующий на 8 ждёт секунду (здесь 50 мс)
// и получает отказ: в очередь из него не попало ни одно событие, id освобождены и повтор принят.
func TestProcessBackpressureAllOrNothing(t *testing.T) {
	ids := newFakeIDs()
	in := testIntake(ids, 10)
	ctx := context.Background()

	if out, err := in.process(ctx, strings.NewReader(eventsBody("a", 8))); err != nil || out.accepted != 8 {
		t.Fatalf("первый запрос: %+v, %v", out, err)
	}
	t0 := time.Now()
	_, err := in.process(ctx, strings.NewReader(eventsBody("b", 8)))
	if !errors.Is(err, errEventsBusy) {
		t.Fatalf("второй запрос: %v, ждали errEventsBusy", err)
	}
	if d := time.Since(t0); d < in.wait {
		t.Errorf("отказ через %v, раньше срока ожидания %v", d, in.wait)
	}
	if got := len(in.w.in); got != 8 {
		t.Errorf("в очереди %d событий, ждали 8: из отклонённого запроса ничего не должно попасть", got)
	}
	if ids.backs != 1 || len(ids.seen) != 8 {
		t.Errorf("откатов %d, занятых id %d; ждали 1 и 8", ids.backs, len(ids.seen))
	}

	// Очередь освободилась (писатель забрал пачку): тот же запрос принят целиком.
	for len(in.w.in) > 0 {
		<-in.w.in
	}
	if out, err := in.process(ctx, strings.NewReader(eventsBody("b", 8))); err != nil || out.accepted != 8 || out.duplicate != 0 {
		t.Fatalf("повтор после освобождения: %+v, %v", out, err)
	}
}

// Место освободилось за время ожидания: запрос ждёт и проходит.
func TestOfferAllWaitsForRoom(t *testing.T) {
	in := testIntake(newFakeIDs(), 4)
	in.wait = 2 * time.Second
	evs := make([]StreamEvent, 3)
	if !in.w.offerAll(context.Background(), evs, time.Second) {
		t.Fatal("первые три не встали")
	}
	go func() {
		time.Sleep(30 * time.Millisecond)
		<-in.w.in
		<-in.w.in
	}()
	if !in.w.offerAll(context.Background(), make([]StreamEvent, 3), time.Second) {
		t.Fatal("после освобождения запрос не прошёл")
	}
	if got := len(in.w.in); got != 4 {
		t.Errorf("в очереди %d, ждали 4", got)
	}
}

func TestOfferAllTooBigForQueue(t *testing.T) {
	in := testIntake(newFakeIDs(), 4)
	if in.w.offerAll(context.Background(), make([]StreamEvent, 5), time.Second) {
		t.Error("запрос больше очереди принят")
	}
}

func TestPostSourceEvents503RetryAfter(t *testing.T) {
	a := &SourceAPI{intake: testIntake(newFakeIDs(), 3)}
	a.intakeOnce.Do(func() {})
	if rec := postEvents(a, eventsBody("a", 3)); rec.Code != http.StatusAccepted {
		t.Fatalf("первый: %d %s", rec.Code, rec.Body)
	}
	rec := postEvents(a, eventsBody("b", 2))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("при полной очереди: %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("нет Retry-After")
	}
	if got := len(a.intake.w.in); got != 3 {
		t.Errorf("в очереди %d, ждали 3", got)
	}
}

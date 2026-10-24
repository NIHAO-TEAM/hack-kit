package source

import (
	"context"
	"strings"
	"testing"
)

func TestParseEventRejects(t *testing.T) {
	st := newOpenIngestState()
	cases := map[string]string{
		`not json`: "не разобрано",
		`{"kind":"top_up","accountId":1,"balance":5}`:                                                           "eventId обязателен",
		`{"eventId":"a","kind":"refund","accountId":1,"balance":5}`:                                             "неизвестный вид",
		`{"eventId":"a","kind":"top_up","accountId":1}`:                                                         "ничего не меняет",
		`{"eventId":"a","kind":"top_up","balance":5}`:                                                           "нужен accountId",
		`{"eventId":"a","kind":"top_up","accountId":1,"balance":-1}`:                                            "отрицательная сумма",
		`{"eventId":"a","kind":"service_stop","status":"suspended"}`:                                            "нужен serviceId",
		`{"eventId":"a","kind":"service_stop","serviceId":2}`:                                                   "ничего не меняет",
		`{"eventId":"a","kind":"service_stop","serviceId":2,"status":"x"}`:                                      "неизвестный статус",
		`{"eventId":"a","kind":"top_up","serviceId":2,"status":"active"}`:                                       "top_up: нужен balance",
		`{"eventId":"a","kind":"autoprolong_on","serviceId":2,"status":"active"}`:                               "нужен autoProlong",
		`{"eventId":"` + strings.Repeat("x", eventsMaxIDLen+1) + `","kind":"top_up","accountId":1,"balance":5}`: "длиннее",
	}
	for body, want := range cases {
		_, err := parseEvent(rawEvent{line: 1, data: []byte(body)}, st)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: ошибка %v, ждали %q", body, err, want)
		}
	}
}

func TestParseEventKinds(t *testing.T) {
	st := newOpenIngestState()
	for _, body := range []string{
		`{"eventId":"1","kind":"top_up","accountId":1,"balance":500}`,
		`{"eventId":"2","kind":"autoprolong_on","serviceId":2,"autoProlong":true}`,
		`{"eventId":"3","kind":"autoprolong_off","serviceId":2,"autoProlong":false}`,
		`{"eventId":"4","kind":"service_stop","serviceId":2,"status":"suspended"}`,
		`{"eventId":"5","kind":"service_resume","serviceId":2,"status":"active"}`,
		`{"eventId":"6","kind":"renewal_failed","serviceId":2,"status":"prolong_wait","expiresAt":"2026-10-20T00:00:00Z","accountId":1,"balance":0}`,
		`{"eventId":"7","kind":"prolong_wait","serviceId":2,"status":"prolong_wait"}`,
	} {
		if _, err := parseEvent(rawEvent{line: 1, data: []byte(body)}, st); err != nil {
			t.Errorf("%s: %v", body, err)
		}
	}
}

// Строки с ошибкой попадают в rejected со своим номером, остальные принимаются.
func TestProcessPartialRejected(t *testing.T) {
	in := testIntake(newFakeIDs(), 16)
	body := strings.Join([]string{
		`{"eventId":"a","kind":"top_up","accountId":1,"balance":100}`,
		`{"eventId":"b","kind":"top_up","accountId":1}`,
		``,
		`garbage`,
		`{"eventId":"c","kind":"service_stop","serviceId":3,"status":"suspended"}`,
	}, "\n")
	out, err := in.process(context.Background(), strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if out.accepted != 2 || out.duplicate != 0 || len(out.rejected) != 2 {
		t.Fatalf("итог %+v", out)
	}
	if out.rejected[0].Line != 2 || out.rejected[1].Line != 4 {
		t.Errorf("номера строк %d и %d, ждали 2 и 4", out.rejected[0].Line, out.rejected[1].Line)
	}
	if out.version != 7 {
		t.Errorf("версия %d", out.version)
	}
	if got := len(in.w.in); got != 2 {
		t.Errorf("в очереди %d событий, ждали 2", got)
	}
}

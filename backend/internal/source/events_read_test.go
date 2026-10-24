package source

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestReadEventsNDJSON(t *testing.T) {
	body := "{\"eventId\":\"a\"}\n\n  {\"eventId\":\"b\"}  \r\n{\"eventId\":\"c\"}"
	got, err := readEvents(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("событий %d, ждали 3", len(got))
	}
	// Номер — физическая строка тела, пустые строки его не сдвигают.
	for i, wantLine := range []int{1, 3, 4} {
		if got[i].line != wantLine {
			t.Errorf("событие %d: строка %d, ждали %d", i, got[i].line, wantLine)
		}
	}
	if string(got[1].data) != `{"eventId":"b"}` {
		t.Errorf("данные второй строки: %q", got[1].data)
	}
}

func TestReadEventsArray(t *testing.T) {
	got, err := readEvents(strings.NewReader("\n [ {\"eventId\":\"a\"},\n{\"eventId\":\"b\"} ]"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].line != 1 || got[1].line != 2 {
		t.Fatalf("разобрано %+v", got)
	}
}

func TestReadEventsEmptyBody(t *testing.T) {
	for _, body := range []string{"", " \n\t"} {
		got, err := readEvents(strings.NewReader(body))
		if err != nil || len(got) != 0 {
			t.Errorf("тело %q: %v, %v", body, got, err)
		}
	}
}

func TestReadEventsBrokenArray(t *testing.T) {
	for _, body := range []string{`[{"eventId":"a"},`, `[{"eventId":"a"} {"eventId":"b"}]`, `[1,`} {
		_, err := readEvents(strings.NewReader(body))
		var be eventsBodyError
		if !errors.As(err, &be) {
			t.Errorf("тело %q: ошибка %v, ждали eventsBodyError", body, err)
		}
	}
}

// Предел числа событий: ровно eventsMaxCount проходит, на одно больше — errEventsTooMany, в обоих форматах.
func TestReadEventsCountLimit(t *testing.T) {
	nd := func(n int) string { return strings.Repeat("{\"eventId\":\"x\"}\n", n) }
	arr := func(n int) string {
		return "[" + strings.TrimSuffix(strings.Repeat(`{"eventId":"x"},`, n), ",") + "]"
	}
	for name, mk := range map[string]func(int) string{"ndjson": nd, "array": arr} {
		got, err := readEvents(strings.NewReader(mk(eventsMaxCount)))
		if err != nil || len(got) != eventsMaxCount {
			t.Errorf("%s, %d событий: %d, %v", name, eventsMaxCount, len(got), err)
		}
		if _, err := readEvents(strings.NewReader(mk(eventsMaxCount + 1))); !errors.Is(err, errEventsTooMany) {
			t.Errorf("%s, на одно больше предела: %v", name, err)
		}
	}
}

func TestReadEventsLongLine(t *testing.T) {
	_, err := readEvents(strings.NewReader(`{"eventId":"` + strings.Repeat("a", eventsMaxLine) + `"}`))
	var be eventsBodyError
	if !errors.As(err, &be) {
		t.Fatalf("слишком длинная строка: %v", err)
	}
}

// Предел тела 2 МБ даёт ошибку чтения, которую обработчик отвечает 413.
func TestReadEventsBodyLimit(t *testing.T) {
	line := fmt.Sprintf("{\"eventId\":\"%s\"}\n", strings.Repeat("a", 1000))
	for name, body := range map[string]string{
		"ndjson": strings.Repeat(line, eventsMaxBytes/len(line)+10),
		"array":  "[" + strings.TrimSuffix(strings.Repeat(strings.TrimSpace(line)+",", eventsMaxBytes/len(line)+10), ",") + "]",
	} {
		rec := &limitedBody{r: strings.NewReader(body), left: eventsMaxBytes}
		_, err := readEvents(rec)
		var tooBig *http.MaxBytesError
		if !errors.As(err, &tooBig) {
			t.Errorf("%s: ошибка %v, ждали MaxBytesError", name, err)
		}
	}
}

// limitedBody — читатель с пределом, как http.MaxBytesReader, но без ResponseWriter.
type limitedBody struct {
	r    *strings.Reader
	left int64
}

func (l *limitedBody) Read(p []byte) (int, error) {
	if l.left <= 0 {
		return 0, &http.MaxBytesError{Limit: eventsMaxBytes}
	}
	if int64(len(p)) > l.left {
		p = p[:l.left]
	}
	n, err := l.r.Read(p)
	l.left -= int64(n)
	return n, err
}

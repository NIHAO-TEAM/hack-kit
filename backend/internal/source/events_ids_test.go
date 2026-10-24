package source

import (
	"context"
	"strings"
	"testing"
)

func TestProcessDuplicates(t *testing.T) {
	ids := newFakeIDs()
	in := testIntake(ids, 16)
	body := `{"eventId":"a","kind":"top_up","accountId":1,"balance":100}` + "\n" +
		`{"eventId":"b","kind":"top_up","accountId":2,"balance":100}` + "\n" +
		`{"eventId":"a","kind":"top_up","accountId":1,"balance":999}`

	// Повтор внутри запроса считается дублем, применяется первое вхождение.
	out, err := in.process(context.Background(), strings.NewReader(body))
	if err != nil || out.accepted != 2 || out.duplicate != 1 {
		t.Fatalf("первый запрос: %+v, %v", out, err)
	}
	// Тот же запрос второй раз: всё дубли, в очередь ничего не добавлено.
	out, err = in.process(context.Background(), strings.NewReader(body))
	if err != nil || out.accepted != 0 || out.duplicate != 3 {
		t.Fatalf("повтор запроса: %+v, %v", out, err)
	}
	if got := len(in.w.in); got != 2 {
		t.Errorf("в очереди %d событий, ждали 2", got)
	}
	// Новый id вместе со старым: старый — дубль.
	out, err = in.process(context.Background(), strings.NewReader(
		`{"eventId":"a","kind":"top_up","accountId":1,"balance":1}`+"\n"+`{"eventId":"c","kind":"top_up","accountId":1,"balance":1}`))
	if err != nil || out.accepted != 1 || out.duplicate != 1 {
		t.Fatalf("смешанный запрос: %+v, %v", out, err)
	}
	if ids.commits != 3 {
		t.Errorf("зафиксировано %d занятий id, ждали 3", ids.commits)
	}
}

// Запрос без годных событий базу id не трогает.
func TestProcessNothingValid(t *testing.T) {
	ids := newFakeIDs()
	in := testIntake(ids, 16)
	out, err := in.process(context.Background(), strings.NewReader(`{"kind":"top_up"}`))
	if err != nil || out.accepted != 0 || len(out.rejected) != 1 || ids.claims != 0 {
		t.Fatalf("%+v, %v, занятий %d", out, err, ids.claims)
	}
}

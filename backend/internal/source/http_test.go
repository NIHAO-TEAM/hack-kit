package source_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"example.com/app/internal/apitest"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/source"
)

const organizersPath = "../../../cases/analizator/data.json"

func put(t *testing.T, c *gen.ClientWithResponses, body []byte) *gen.PutSnapshotResponse {
	t.Helper()
	res, err := c.PutSnapshotWithBodyWithResponse(context.Background(), "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func status(t *testing.T, c *gen.ClientWithResponses) *gen.SourceStatus {
	t.Helper()
	res, err := c.GetSourceStatusWithResponse(context.Background())
	if err != nil || res.JSON200 == nil {
		t.Fatalf("статус: %v %v", err, res)
	}
	return res.JSON200
}

func organizers(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(organizersPath)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

const oneAccount = `{"accounts":[{"id":7,"email":"a@x.test","balance":1,"withdrawBalance":0,"totalReplenished":1,"bonusBalance":0,"bonusState":"locked"}],"services":[],"transactions":[]}`

func countRows(t *testing.T, s *apitest.Stand) [3]int {
	t.Helper()
	var n [3]int
	for i, tbl := range []string{"accounts", "services", "transactions"} {
		if err := s.Pool.QueryRow(context.Background(), "SELECT count(*) FROM "+tbl).Scan(&n[i]); err != nil {
			t.Fatal(err)
		}
	}
	return n
}

func TestPutSnapshotOrganizers(t *testing.T) {
	s := apitest.New(t)
	events, cancel := s.Bus.Subscribe(source.TopicChanged)
	defer cancel()
	c := s.As(t, "analyst")

	res := put(t, c, organizers(t))
	if res.JSON200 == nil {
		t.Fatalf("ждали 200: %d %s", res.StatusCode(), res.Body)
	}
	got := res.JSON200
	if got.Accounts != 10 || got.Services != 30 || got.Transactions != 100 {
		t.Errorf("счётчики %d/%d/%d", got.Accounts, got.Services, got.Transactions)
	}
	if want := time.Date(2026, 10, 19, 14, 35, 53, 0, time.UTC); !got.AsOf.Equal(want) {
		t.Errorf("asOf %v, ждали %v", got.AsOf, want)
	}
	st := status(t, c)
	if st.Version != got.Version || st.Source == nil || *st.Source != gen.Api {
		t.Errorf("статус %+v", st)
	}
	select {
	case ev := <-events:
		if ev.Payload != got.Version {
			t.Errorf("событие несёт %v, ждали %d", ev.Payload, got.Version)
		}
	case <-time.After(time.Second):
		t.Error("source.changed не опубликован")
	}
}

func TestPutSnapshotReplaces(t *testing.T) {
	s := apitest.New(t)
	c := s.As(t, "analyst")
	first := put(t, c, organizers(t))
	second := put(t, c, []byte(oneAccount))
	if second.JSON200 == nil || second.JSON200.Accounts != 1 || second.JSON200.Version <= first.JSON200.Version {
		t.Fatalf("замена: %d %s", second.StatusCode(), second.Body)
	}
	if n := countRows(t, s); n != [3]int{1, 0, 0} {
		t.Errorf("прежние строки не заменены: %v", n)
	}
}

func TestPutSnapshotAsOfFromBody(t *testing.T) {
	s := apitest.New(t)
	c := s.As(t, "analyst")
	body := strings.Replace(oneAccount, `{"accounts"`, `{"asOf":"2026-10-19T15:00:00Z","accounts"`, 1)
	res := put(t, c, []byte(body))
	if res.JSON200 == nil {
		t.Fatalf("%d %s", res.StatusCode(), res.Body)
	}
	want := time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC)
	if st := status(t, c); st.AsOf == nil || !st.AsOf.Equal(want) {
		t.Errorf("asOf в статусе %v", st.AsOf)
	}
}

func TestPutSnapshotInvalidKeepsOld(t *testing.T) {
	s := apitest.New(t)
	c := s.As(t, "analyst")
	ok := put(t, c, organizers(t))
	bad := strings.Replace(string(organizers(t)), `"ownerId": 1,`, `"ownerId": 999,`, 1)
	if bad == string(organizers(t)) {
		t.Fatal("в data.json не нашлось ownerId: 1")
	}
	res := put(t, c, []byte(bad))
	if res.ApplicationproblemJSON422 == nil && res.StatusCode() != http.StatusUnprocessableEntity {
		t.Fatalf("ждали 422: %d %s", res.StatusCode(), res.Body)
	}
	var p gen.Problem
	if err := json.Unmarshal(res.Body, &p); err != nil || p.Errors == nil || len(*p.Errors) == 0 {
		t.Fatalf("тело 422: %v %s", err, res.Body)
	}
	if f := (*p.Errors)[0].Field; !strings.HasSuffix(f, ".ownerId") {
		t.Errorf("поле ошибки %q", f)
	}
	if st := status(t, c); st.Version != ok.JSON200.Version {
		t.Errorf("версия изменилась: %d → %d", ok.JSON200.Version, st.Version)
	}
	if n := countRows(t, s); n != [3]int{10, 30, 100} {
		t.Errorf("прежние данные потеряны: %v", n)
	}
}

func TestPutSnapshotTypos(t *testing.T) {
	s := apitest.New(t)
	typo := string(organizers(t))
	for _, r := range [][2]string{{`"price"`, `"pice"`}, {`"paymentTerm"`, `"paymentTem"`}, {`"autoProlong"`, `"autoprolong"`}, {`"quarter_year"`, `"quater_year"`}, {`"performed"`, `"pefomed"`}} {
		typo = strings.ReplaceAll(typo, r[0], r[1])
	}
	res := put(t, s.As(t, "analyst"), []byte(typo))
	if res.JSON200 == nil || res.JSON200.Services != 30 {
		t.Fatalf("%d %s", res.StatusCode(), res.Body)
	}
}

func TestPutSnapshotBrokenBody(t *testing.T) {
	s := apitest.New(t)
	res := put(t, s.As(t, "analyst"), []byte(`{"accounts":`))
	if res.StatusCode() != http.StatusBadRequest {
		t.Fatalf("ждали 400: %d %s", res.StatusCode(), res.Body)
	}
}

func TestPutSnapshotRoles(t *testing.T) {
	s := apitest.New(t)
	body := []byte(oneAccount)
	cases := []struct {
		name string
		c    *gen.ClientWithResponses
		want int
	}{
		{"аноним", s.Anonymous(t), http.StatusUnauthorized},
		{"маркетолог", s.As(t, "marketer"), http.StatusForbidden},
		{"аналитик", s.As(t, "analyst"), http.StatusOK},
		{"жюри", s.As(t, "jury"), http.StatusOK},
		{"администратор", s.As(t, "admin"), http.StatusOK},
		{"сервисный токен", s.WithBearer(t, apitest.SourceToken), http.StatusOK},
	}
	for _, c := range cases {
		if got := put(t, c.c, body).StatusCode(); got != c.want {
			t.Errorf("%s: %d, ждали %d", c.name, got, c.want)
		}
	}
}

func TestStatusWithoutSnapshot(t *testing.T) {
	s := apitest.New(t)
	st := status(t, s.As(t, "marketer"))
	if st.Version != 0 || st.Source != nil {
		t.Errorf("пустая база: %+v", st)
	}
}

func TestSeed(t *testing.T) {
	s := apitest.New(t)
	// apitest стартует без сида; сид загружаем тем же OnStart, что и cmd/api.
	bus := events.New()
	ch, cancel := bus.Subscribe(source.TopicChanged)
	defer cancel()
	if err := source.OnStart(context.Background(), s.Pool, bus, s.Clock, true); err != nil {
		t.Fatal(err)
	}
	st := status(t, s.As(t, "marketer"))
	want := time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC)
	if st.Source == nil || *st.Source != gen.Seed || st.AsOf == nil || !st.AsOf.Equal(want) {
		t.Fatalf("статус сида %+v", st)
	}
	if n := countRows(t, s); n != [3]int{10, 30, 100} {
		t.Errorf("строк %v", n)
	}
	<-ch
	// Второй старт сид не повторяет.
	if err := source.OnStart(context.Background(), s.Pool, bus, s.Clock, true); err != nil {
		t.Fatal(err)
	}
	if again := status(t, s.As(t, "marketer")); again.Version != st.Version {
		t.Errorf("версия %d → %d", st.Version, again.Version)
	}
	select {
	case <-ch:
		t.Error("второй старт опубликовал событие")
	default:
	}
}

func TestSeedDisabled(t *testing.T) {
	s := apitest.New(t)
	if err := source.OnStart(context.Background(), s.Pool, events.New(), s.Clock, false); err != nil {
		t.Fatal(err)
	}
	if st := status(t, s.As(t, "marketer")); st.Version != 0 {
		t.Errorf("сид загружен при выключенном флаге: %+v", st)
	}
}

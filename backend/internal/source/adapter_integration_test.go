//go:build integration

package source_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"example.com/app/internal/apitest"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/source"
)

// billing — подставной биллинг: тело и код ответа меняются по ходу теста.
type billing struct {
	srv  *httptest.Server
	body atomic.Value // []byte
	code atomic.Int32
	auth atomic.Value // string
	hits atomic.Int32
}

func newBilling(t *testing.T, body []byte) *billing {
	b := &billing{}
	b.body.Store(body)
	b.code.Store(http.StatusOK)
	b.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.hits.Add(1)
		b.auth.Store(r.Header.Get("Authorization"))
		if c := int(b.code.Load()); c != http.StatusOK {
			http.Error(w, "boom", c)
			return
		}
		_, _ = w.Write(b.body.Load().([]byte))
	}))
	t.Cleanup(b.srv.Close)
	return b
}

func standWithBilling(t *testing.T, b *billing) (*apitest.Stand, *source.Adapter) {
	t.Helper()
	return apitest.NewBilling(t, source.AdapterConfig{URL: b.srv.URL, Token: "bill-token", Interval: time.Second})
}

func balanceOf(t *testing.T, s *apitest.Stand, id int64) int64 {
	t.Helper()
	snap, _, err := source.NewReader(s.Pool).Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range snap.Accounts {
		if a.ID == id {
			return a.Balance
		}
	}
	t.Fatalf("клиента %d нет", id)
	return 0
}

func TestPollWritesNewSnapshotAndSkipsSame(t *testing.T) {
	b := newBilling(t, []byte(oneAccount))
	s, ad := standWithBilling(t, b)
	events, cancel := s.Bus.Subscribe(source.TopicChanged)
	defer cancel()
	ctx := context.Background()

	changed, err := ad.Poll(ctx)
	if err != nil || !changed {
		t.Fatalf("первый опрос: changed=%v err=%v", changed, err)
	}
	if got := b.auth.Load(); got != "Bearer bill-token" {
		t.Errorf("Authorization = %v", got)
	}
	select {
	case <-events:
	case <-time.After(time.Second):
		t.Error("source.changed не опубликован")
	}
	st := status(t, s.As(t, "analyst"))
	if st.Source == nil || *st.Source != gen.SourceStatusSource("api") {
		t.Errorf("source = %v, ждали api", st.Source)
	}
	v1 := st.Version

	if changed, err := ad.Poll(ctx); err != nil || changed {
		t.Fatalf("тот же ответ: changed=%v err=%v", changed, err)
	}
	if v := status(t, s.As(t, "analyst")).Version; v != v1 {
		t.Errorf("версия выросла без изменений: %d → %d", v1, v)
	}

	b.body.Store([]byte(strings.Replace(oneAccount, `"balance":1`, `"balance":900`, 1)))
	if changed, err := ad.Poll(ctx); err != nil || !changed {
		t.Fatalf("новый баланс: changed=%v err=%v", changed, err)
	}
	if got := balanceOf(t, s, 7); got != 900 {
		t.Errorf("баланс клиента 7 = %d, ждали 900", got)
	}
}

func TestPollFailureKeepsDataAndCountsErrors(t *testing.T) {
	b := newBilling(t, []byte(oneAccount))
	s, ad := standWithBilling(t, b)
	ctx := context.Background()
	if _, err := ad.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	before := status(t, s.As(t, "analyst"))

	b.code.Store(http.StatusBadGateway)
	if _, err := ad.Poll(ctx); err == nil {
		t.Fatal("ждали ошибку при 502")
	}
	b.code.Store(http.StatusOK)
	b.body.Store([]byte(`{"accounts":`)) // оборванный JSON
	if _, err := ad.Poll(ctx); err == nil {
		t.Fatal("ждали ошибку при битом теле")
	}
	b.body.Store([]byte(`{"accounts":[{"id":7,"email":"","balance":-5}],"services":[],"transactions":[]}`))
	if _, err := ad.Poll(ctx); err == nil {
		t.Fatal("ждали ошибку при снимке, не прошедшем проверку")
	}

	after := status(t, s.As(t, "analyst"))
	if after.Version != before.Version || balanceOf(t, s, 7) != 1 {
		t.Errorf("данные изменились после сбоев: версия %d → %d", before.Version, after.Version)
	}
	if after.AsOf == nil || !after.AsOf.Equal(*before.AsOf) {
		t.Errorf("asOf сдвинулся: %v → %v", before.AsOf, after.AsOf)
	}
	ps := ad.Status()
	if ps.ErrorsTotal != 3 || ps.LastError == "" {
		t.Errorf("статус опроса: %+v", ps)
	}

	b.body.Store([]byte(oneAccount))
	if _, err := ad.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	if ps := ad.Status(); ps.LastError != "" || ps.LastOKAt == nil || ps.ErrorsTotal != 3 {
		t.Errorf("после восстановления: %+v", ps)
	}
}

func TestMetricsCountPollErrors(t *testing.T) {
	b := newBilling(t, nil)
	b.code.Store(http.StatusInternalServerError)
	s, ad := standWithBilling(t, b)
	_, _ = ad.Poll(context.Background())
	_, _ = ad.Poll(context.Background())

	req, _ := http.NewRequest(http.MethodGet, s.URL+"/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+apitest.MetricsToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(resp.Body)
	if !strings.Contains(buf.String(), "churn_billing_poll_errors_total 2") {
		t.Errorf("в /metrics нет churn_billing_poll_errors_total 2:\n%s", buf.String())
	}
}

func TestManualPutPausesPollingUntilResumed(t *testing.T) {
	b := newBilling(t, []byte(strings.Replace(oneAccount, `"balance":1`, `"balance":500`, 1)))
	s, ad := standWithBilling(t, b)
	c := s.As(t, "analyst")
	ctx := context.Background()

	if changed, err := ad.Poll(ctx); err != nil || !changed {
		t.Fatalf("опрос до PUT: %v %v", changed, err)
	}
	if res := put(t, c, []byte(oneAccount)); res.JSON200 == nil {
		t.Fatalf("PUT: %d %s", res.StatusCode(), res.Body)
	}
	if st := status(t, c); st.Polling == nil || !st.Polling.Paused {
		t.Fatalf("после PUT опрос не на паузе: %+v", st.Polling)
	}
	if balanceOf(t, s, 7) != 1 {
		t.Fatal("ручной снимок не записан")
	}

	// Run на паузе не ходит в биллинг.
	hits := b.hits.Load()
	runCtx, stop := context.WithTimeout(ctx, 300*time.Millisecond)
	defer stop()
	ad.Run(runCtx)
	if b.hits.Load() != hits {
		t.Error("опрос на паузе сходил в биллинг")
	}

	res, err := c.SetSourcePollingWithResponse(ctx, gen.SourcePollingRequest{Paused: false})
	if err != nil || res.JSON200 == nil || res.JSON200.Paused {
		t.Fatalf("включение опроса: %v %+v", err, res)
	}
	// Тело биллинга то же, но данные в базе заменены вручную: опрос обязан вернуть биллинг.
	if changed, err := ad.Poll(ctx); err != nil || !changed {
		t.Fatalf("опрос после включения: %v %v", changed, err)
	}
	if got := balanceOf(t, s, 7); got != 500 {
		t.Errorf("баланс = %d, ждали 500 из биллинга", got)
	}
}

func TestPollingEndpointWithoutAdapterIs404(t *testing.T) {
	s := apitest.New(t)
	res, err := s.As(t, "analyst").SetSourcePollingWithResponse(context.Background(), gen.SourcePollingRequest{Paused: true})
	if err != nil || res.StatusCode() != http.StatusNotFound {
		t.Fatalf("ждали 404, получили %v %v", res, err)
	}
	if st := status(t, s.As(t, "analyst")); st.Polling != nil {
		t.Errorf("polling без BILLING_URL: %+v", st.Polling)
	}
	if res, _ := s.As(t, "marketer").SetSourcePollingWithResponse(context.Background(), gen.SourcePollingRequest{Paused: true}); res.StatusCode() != http.StatusForbidden {
		t.Errorf("маркетолог: %d, ждали 403", res.StatusCode())
	}
}

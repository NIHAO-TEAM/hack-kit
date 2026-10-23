//go:build integration

package app_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"example.com/app/internal/apitest"
)

// Сводка и метрики на сиде организаторов: уровни 3/2/1/0/4, суммы из specs/metrics.md.
func TestSummaryAndMetricsOnSeed(t *testing.T) {
	r := newRig(t, true)
	rc := r.start(t, r.Pool)
	waitLatest(t, rc, "пересчёт при старте", func() bool { return rc.Engine.Latest() != nil })
	o := rc.Overview()
	want := map[string]int{"high": 3, "medium": 2, "low": 1, "unscored": 0, "lost": 4}
	for k, v := range want {
		if o.Levels[k] != v {
			t.Errorf("уровень %s = %d, ждали %d", k, o.Levels[k], v)
		}
	}
	if o.ExpectedLossCents != 387973 || o.AtRiskCents != 690046 || o.LostCents != 736834 {
		t.Errorf("суммы: потеря %d, под угрозой %d, потеряно %d", o.ExpectedLossCents, o.AtRiskCents, o.LostCents)
	}
}

// Токен сборщика открывает /metrics и /metics; ответ в формате Prometheus.
func TestMetricsEndpointWithToken(t *testing.T) {
	r := newRig(t, true)
	for _, path := range []string{"/metrics", "/metics"} {
		req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, r.URL+path, nil)
		req.Header.Set("Authorization", "Bearer "+apitest.MetricsToken)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if res.StatusCode != 200 || !strings.Contains(string(body), "go_goroutines") {
			t.Errorf("%s: статус %d", path, res.StatusCode)
		}
	}
}

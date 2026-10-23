//go:build integration

package app_test

import (
	"bytes"
	"context"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"example.com/app/internal/apitest"
	"example.com/app/internal/auth"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/tools/snapgen"
)

// metricValue читает значение метрики из текста Prometheus; label — подстрока вида level="high".
func metricValue(t *testing.T, text, name, label string) float64 {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, name) || !strings.Contains(line, label) {
			continue
		}
		f := strings.Fields(line)
		v, err := strconv.ParseFloat(f[len(f)-1], 64)
		if err != nil {
			t.Fatalf("метрика %s: %v", line, err)
		}
		return v
	}
	t.Fatalf("метрики %s %s нет", name, label)
	return 0
}

// Крайние наборы чужого снимка: после загрузки очередь, сводка и /metrics согласованы,
// скор в [0, 100] или null, отрицательных потерь нет.
func TestEdgeSnapshotsConsistent(t *testing.T) {
	r := newRig(t, false)
	r.StartRecalc(t)
	ctx := context.Background()
	marketer := r.As(t, auth.RoleMarketer)
	loader := r.WithBearer(t, apitest.SourceToken)
	lastVersion := int64(0)

	for _, edge := range append([]string{snapgen.EdgeNone}, snapgen.Edges...) {
		t.Run("edge="+edge, func(t *testing.T) {
			body, err := snapgen.Generate(snapgen.Options{Accounts: 150, Seed: 5, Edge: edge})
			if err != nil {
				t.Fatal(err)
			}
			put, err := loader.PutSnapshotWithBodyWithResponse(ctx, "application/json", bytes.NewReader(body))
			if err != nil || put.JSON200 == nil {
				t.Fatalf("PUT: %v %d %s", err, put.StatusCode(), put.Body)
			}

			var sum *gen.Summary
			deadline := time.Now().Add(15 * time.Second)
			for sum == nil {
				res, err := marketer.GetSummaryWithResponse(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if res.JSON200 != nil && res.JSON200.Version > lastVersion {
					sum = res.JSON200
				} else if time.Now().After(deadline) {
					t.Fatalf("пересчёт не завершился: %d %s", res.StatusCode(), res.Body)
				} else {
					time.Sleep(20 * time.Millisecond)
				}
			}
			lastVersion = sum.Version

			var rows []gen.CustomerRow
			for offset := 0; ; offset += 200 {
				limit := 200
				res, err := marketer.ListCustomersWithResponse(ctx, &gen.ListCustomersParams{Limit: &limit, Offset: &offset})
				if err != nil || res.JSON200 == nil {
					t.Fatalf("клиенты: %v %d %s", err, res.StatusCode(), res.Body)
				}
				if res.JSON200.Version != sum.Version {
					t.Skipf("пересчёт ушёл вперёд во время чтения: %d → %d", sum.Version, res.JSON200.Version)
				}
				rows = append(rows, res.JSON200.Items...)
				if len(rows) >= res.JSON200.Total {
					break
				}
			}
			if len(rows) != put.JSON200.Accounts {
				t.Errorf("в очереди %d клиентов, загружено аккаунтов %d", len(rows), put.JSON200.Accounts)
			}

			var loss, atRisk, lost int64
			levels := map[string]int{}
			for _, c := range rows {
				if c.Score != nil && (math.IsNaN(*c.Score) || *c.Score < 0 || *c.Score > 100) {
					t.Errorf("клиент %d: скор %v", c.Id, *c.Score)
				}
				if c.ExpectedLossCents < 0 {
					t.Errorf("клиент %d: потеря %d", c.Id, c.ExpectedLossCents)
				}
				loss += c.ExpectedLossCents
				levels[string(c.Level)]++
			}
			if loss != sum.ExpectedLossCents {
				t.Errorf("сумма потерь очереди %d ≠ сводка %d", loss, sum.ExpectedLossCents)
			}
			for lvl, n := range map[string]int{
				"high": sum.Levels.High, "medium": sum.Levels.Medium, "low": sum.Levels.Low,
				"unscored": sum.Levels.Unscored, "lost": sum.Levels.Lost,
			} {
				if levels[lvl] != n {
					t.Errorf("уровень %s: очередь %d, сводка %d", lvl, levels[lvl], n)
				}
			}
			atRisk, lost = sum.AtRiskCents, sum.LostCents
			if sum.ExpectedLossCents < 0 || atRisk < 0 || lost < 0 {
				t.Errorf("отрицательная сводка: %d/%d/%d", sum.ExpectedLossCents, atRisk, lost)
			}

			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, r.URL+"/metrics", nil)
			req.Header.Set("Authorization", "Bearer "+apitest.MetricsToken)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			text, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()
			m := string(text)
			if got := metricValue(t, m, "churn_model_version", ""); int64(got) != sum.Version {
				t.Skipf("пересчёт ушёл вперёд к чтению метрик: %v → %d", got, sum.Version)
			}
			for name, want := range map[string]int64{
				"churn_expected_loss_cents": sum.ExpectedLossCents, "churn_at_risk_cents": atRisk, "churn_lost_cents": lost,
			} {
				if got := metricValue(t, m, name, ""); int64(got) != want {
					t.Errorf("%s = %v, в сводке %d", name, got, want)
				}
			}
			for lvl, n := range levels {
				if got := metricValue(t, m, "churn_customers", `level="`+lvl+`"`); int(got) != n {
					t.Errorf("churn_customers{%s} = %v, в очереди %d", lvl, got, n)
				}
			}
		})
	}
}

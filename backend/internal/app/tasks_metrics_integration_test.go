//go:build integration

package app_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil/promlint"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"

	"example.com/app/internal/apitest"
	"example.com/app/internal/auth"
	"example.com/app/internal/platform/api/gen"
)

// metricValues читает /metrics и возвращает значения метрик задач: ключ — имя с метками, например
// `churn_retention_tasks{status="in_work"}`.
func metricValues(t *testing.T, r *rig) (map[string]float64, string) {
	t.Helper()
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, r.URL+"/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+apitest.MetricsToken)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("/metrics: %d", res.StatusCode)
	}
	parser := expfmt.NewTextParser(model.UTF8Validation)
	fams, err := parser.TextToMetricFamilies(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("формат Prometheus: %v", err)
	}
	out := map[string]float64{}
	for name, f := range fams {
		if !strings.HasPrefix(name, "churn_retention_") {
			continue
		}
		for _, m := range f.GetMetric() {
			key := name
			if len(m.GetLabel()) > 0 {
				key += "{" + m.GetLabel()[0].GetName() + `="` + m.GetLabel()[0].GetValue() + `"}`
			}
			if f.GetType().String() == "COUNTER" {
				out[key] = m.GetCounter().GetValue()
			} else {
				out[key] = m.GetGauge().GetValue()
			}
		}
	}
	return out, string(raw)
}

func TestTaskMetrics(t *testing.T) {
	r := taskRig(t)
	ctx := context.Background()
	marketer := r.As(t, auth.RoleMarketer)

	// Задач нет: все ряды на месте и равны нулю.
	m, raw := metricValues(t, r)
	for _, k := range []string{
		`churn_retention_tasks{status="in_work"}`, `churn_retention_tasks{status="contacted"}`, `churn_retention_tasks{status="postponed"}`,
		"churn_retention_tasks_overdue", `churn_retention_tasks_closed_total{outcome="renewed"}`,
		`churn_retention_tasks_closed_total{outcome="not_needed"}`, "churn_retention_expected_loss_in_work_cents",
	} {
		if v, ok := m[k]; !ok || v != 0 {
			t.Errorf("без задач %s = %v (есть: %v), ждали 0", k, v, ok)
		}
	}
	problems, err := promlint.New(strings.NewReader(raw)).Lint()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range problems {
		if strings.HasPrefix(p.Metric, "churn_") {
			t.Errorf("promlint: %s: %s", p.Metric, p.Text)
		}
	}

	// Клиенты 9 и 1 в работе, 3 — связались, 4 — взят и закрыт «Продлил».
	t9, t1, t3, t4 := take(t, marketer, 9).JSON201, take(t, marketer, 1).JSON201, take(t, marketer, 3).JSON201, take(t, marketer, 4).JSON201
	if t9 == nil || t1 == nil || t3 == nil || t4 == nil {
		t.Fatal("задачи не созданы")
	}
	if res := patchTask(t, marketer, t3.Id, gen.TaskPatch{Version: t3.Version, Status: statusPtr(gen.Contacted)}); res.JSON200 == nil {
		t.Fatalf("связались: %s", res.Body)
	}
	if res := patchTask(t, marketer, t4.Id, gen.TaskPatch{Version: t4.Version, Status: statusPtr(gen.Closed), Outcome: outcomePtr(gen.Renewed)}); res.JSON200 == nil {
		t.Fatalf("закрытие: %s", res.Body)
	}

	// Потеря под работой = Σ потерь клиентов 9, 1, 3 из API.
	var wantLoss int64
	for _, id := range []int64{9, 1, 3} {
		card, err := marketer.GetCustomerWithResponse(ctx, id)
		if err != nil || card.JSON200 == nil {
			t.Fatalf("карточка %d: %v", id, err)
		}
		wantLoss += card.JSON200.Customer.ExpectedLossCents
	}
	m, _ = metricValues(t, r)
	for k, want := range map[string]float64{
		`churn_retention_tasks{status="in_work"}`:               2,
		`churn_retention_tasks{status="contacted"}`:             1,
		`churn_retention_tasks{status="postponed"}`:             0,
		"churn_retention_tasks_overdue":                         0,
		`churn_retention_tasks_closed_total{outcome="renewed"}`: 1,
		`churn_retention_tasks_closed_total{outcome="refused"}`: 0,
		"churn_retention_expected_loss_in_work_cents":           float64(wantLoss),
	} {
		if m[k] != want {
			t.Errorf("%s = %v, ждали %v", k, m[k], want)
		}
	}

	// Сверка с экраном «Задачи» и очередью: просроченные, по статусу и открытые по фильтру.
	r.Clock.Advance(72 * time.Hour)
	marketer = r.As(t, auth.RoleMarketer) // сессия прежнего клиента за трое суток истекла
	m, _ = metricValues(t, r)
	overdue := true
	list, err := marketer.ListTasksWithResponse(ctx, &gen.ListTasksParams{Overdue: &overdue})
	if err != nil || list.JSON200 == nil {
		t.Fatalf("список просроченных: %v", err)
	}
	if got := m["churn_retention_tasks_overdue"]; got != float64(list.JSON200.Total) || got != 3 {
		t.Errorf("просрочено в метриках %v, на экране %d, ждали 3", got, list.JSON200.Total)
	}
	open := gen.ListCustomersParamsTask("open")
	queue, err := marketer.ListCustomersWithResponse(ctx, &gen.ListCustomersParams{Task: &open})
	if err != nil || queue.JSON200 == nil {
		t.Fatalf("очередь: %v", err)
	}
	if got := m[`churn_retention_tasks{status="in_work"}`] + m[`churn_retention_tasks{status="contacted"}`] + m[`churn_retention_tasks{status="postponed"}`]; got != float64(queue.JSON200.Total) {
		t.Errorf("открытых в метриках %v, в очереди %d", got, queue.JSON200.Total)
	}

	// Клиент 9 пропал из снимка, у клиента 1 другой email: их потеря не считается.
	r.reloadWith(t, func(s map[string][]map[string]any) {
		var services, accounts, txs []map[string]any
		gone := map[float64]bool{}
		for _, sv := range s["services"] {
			if sv["ownerId"] == float64(9) {
				gone[sv["id"].(float64)] = true
				continue
			}
			services = append(services, sv)
		}
		for _, a := range s["accounts"] {
			switch a["id"] {
			case float64(9):
				continue
			case float64(1):
				a["email"] = "someone-else@example.test"
			}
			accounts = append(accounts, a)
		}
		for _, tx := range s["transactions"] {
			if !gone[tx["serviceId"].(float64)] {
				txs = append(txs, tx)
			}
		}
		s["accounts"], s["services"], s["transactions"] = accounts, services, txs
	})
	eventually(t, "клиент 9 пропал из портфеля", func() bool {
		res, _ := marketer.GetCustomerWithResponse(ctx, 9)
		return res.StatusCode() == http.StatusNotFound
	})
	card3, _ := marketer.GetCustomerWithResponse(ctx, 3)
	if card3.JSON200 == nil {
		t.Fatalf("карточка 3: %s", card3.Body)
	}
	m, _ = metricValues(t, r)
	if got, want := m["churn_retention_expected_loss_in_work_cents"], float64(card3.JSON200.Customer.ExpectedLossCents); got != want {
		t.Errorf("потеря после замены снимка = %v, ждали %v (только клиент 3)", got, want)
	}
	if m[`churn_retention_tasks{status="in_work"}`] != 2 {
		t.Errorf("задачи не должны пропадать вместе со снимком: %v", m)
	}
}

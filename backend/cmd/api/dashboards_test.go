package main

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"

	"example.com/app/internal/app"
	"example.com/app/internal/auth"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
)

// metricsKnown — имена метрик сервера. go_* и process_* берутся из живого /metrics, churn_* — из
// строковых литералов с именами в коде (ряды без наблюдений и метрики с базой в /metrics без базы
// не видны), плюс хвосты гистограмм.
func metricsKnown(t *testing.T) map[string]bool {
	t.Helper()
	h := app.New(nil, events.New(), clock.System{}, auth.New(nil, clock.System{}, auth.Config{MetricsToken: "t"}), nil)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer t")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/metrics: %d", rec.Code)
	}
	known := map[string]bool{}
	add := func(name string) {
		known[name] = true
		for _, s := range []string{"_bucket", "_count", "_sum"} {
			known[name+s] = true
		}
	}
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if f := strings.Fields(line); len(f) >= 3 && f[0] == "#" && f[1] == "TYPE" {
			add(f[2])
		}
	}
	literal := regexp.MustCompile(`"(churn_[a-z0-9_]+)"`)
	err := filepath.WalkDir(filepath.Join("..", "..", "internal"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		src, err := os.ReadFile(path)
		for _, m := range literal.FindAllSubmatch(src, -1) {
			add(string(m[1]))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return known
}

var metricName = regexp.MustCompile(`\b(?:go|process|churn)_[a-z0-9_]+\b`)

type dashboard struct {
	Panels []struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Targets     []struct {
			Expr string `json:"expr"`
		} `json:"targets"`
	} `json:"panels"`
}

func loadDashboard(t *testing.T, name string) dashboard {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "deploy", "grafana", "dashboards", name))
	if err != nil {
		t.Fatal(err)
	}
	var d dashboard
	if err := json.Unmarshal(b, &d); err != nil {
		t.Fatalf("%s не разбирается: %v", name, err)
	}
	return d
}

// Каждый запрос дашборда «Внутри сервиса» ссылается на метрику, которую отдаёт сервер; панели про
// метрики, которых ещё нет на main, помечены «нет данных до слияния» в названии.
func TestRuntimeDashboardUsesKnownMetrics(t *testing.T) {
	known := metricsKnown(t)
	for _, p := range loadDashboard(t, "runtime.json").Panels {
		marked := strings.Contains(p.Title, "нет данных до слияния")
		for _, tg := range p.Targets {
			for _, m := range metricName.FindAllString(tg.Expr, -1) {
				if !known[m] && !marked {
					t.Errorf("панель %q: метрики %s нет в /metrics и нет пометки «нет данных до слияния»", p.Title, m)
				}
				if known[m] && marked {
					t.Errorf("панель %q помечена, но метрика %s уже есть: снимите пометку", p.Title, m)
				}
			}
		}
	}
}

// Панели задач убраны из Grafana вместе с интерфейсом задач (ADR-0004, п. 1).
func TestChurnDashboardHasNoTaskPanels(t *testing.T) {
	for _, p := range loadDashboard(t, "churn.json").Panels {
		for _, tg := range p.Targets {
			if strings.Contains(tg.Expr, "churn_retention_") {
				t.Errorf("панель %q читает %s", p.Title, tg.Expr)
			}
		}
		if p.Title == "Работа с клиентами" {
			t.Errorf("остался раздел %q", p.Title)
		}
	}
}

// Правила Prometheus разбираются, у каждого есть выражение и оно ссылается на известные метрики.
func TestPrometheusRulesParse(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "deploy", "prometheus", "rules.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		Groups []struct {
			Name  string `yaml:"name"`
			Rules []struct {
				Alert  string `yaml:"alert"`
				Record string `yaml:"record"`
				Expr   string `yaml:"expr"`
			} `yaml:"rules"`
		} `yaml:"groups"`
	}
	if err := yaml.Unmarshal(b, &f); err != nil {
		t.Fatalf("rules.yml не разбирается: %v", err)
	}
	known := metricsKnown(t)
	seen := map[string]bool{}
	for _, g := range f.Groups {
		for _, r := range g.Rules {
			name := r.Alert + r.Record
			seen[name] = true
			if name == "" || r.Expr == "" {
				t.Errorf("группа %s: правило без имени или выражения", g.Name)
			}
			for _, m := range metricName.FindAllString(r.Expr, -1) {
				if !known[m] && !strings.HasPrefix(m, "churn:") {
					t.Errorf("правило %s: метрики %s нет у сервера", name, m)
				}
			}
		}
	}
	for _, want := range []string{"RecalcSlow", "churn:http_request_duration_seconds:p95_5m"} {
		if !seen[want] {
			t.Errorf("нет правила %s", want)
		}
	}
}

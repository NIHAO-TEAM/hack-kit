package retention

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"example.com/app/internal/platform/api/gen"
)

var (
	descTasks = prometheus.NewDesc("churn_retention_tasks",
		"Открытые задачи удержания по статусу (штуки); status: in_work, contacted, postponed.", []string{"status"}, nil)
	descTasksOverdue = prometheus.NewDesc("churn_retention_tasks_overdue",
		"Открытые задачи удержания с прошедшим сроком (штуки).", nil, nil)
	descTasksClosed = prometheus.NewDesc("churn_retention_tasks_closed_total",
		"Закрытые задачи удержания по итогу (штуки); outcome: renewed, refused, no_answer, not_needed.", []string{"outcome"}, nil)
	descLossInWork = prometheus.NewDesc("churn_retention_expected_loss_in_work_cents",
		"Ожидаемая потеря клиентов, которые сейчас в работе, по текущему пересчёту (центы евро); клиенты вне снимка не считаются.", nil, nil)
)

// collectTimeout ограничивает чтение базы при сборе метрик: Prometheus не должен ждать дольше своего таймаута.
const collectTimeout = 5 * time.Second

var (
	openStatuses = []gen.TaskStatus{gen.InWork, gen.Contacted, gen.Postponed}
	outcomes     = []gen.TaskOutcome{gen.Renewed, gen.Refused, gen.NoAnswer, gen.NotNeeded}
)

// Collector возвращает сборщик метрик задач для /metrics. Числа берутся из базы при каждом сборе,
// поэтому переживают рестарт и совпадают с экраном «Задачи».
func (s *Service) Collector() prometheus.Collector { return taskCollector{s} }

type taskCollector struct{ s *Service }

func (taskCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{descTasks, descTasksOverdue, descTasksClosed, descLossInWork} {
		ch <- d
	}
}

// Collect при ошибке базы пропускает метрики задач: остальной /metrics отдаётся как обычно.
func (c taskCollector) Collect(ch chan<- prometheus.Metric) {
	s := c.s
	if s == nil || s.pool == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), collectTimeout)
	defer cancel()
	byStatus := map[gen.TaskStatus]int{}
	byOutcome := map[gen.TaskOutcome]int{}
	rows, err := s.pool.Query(ctx, `SELECT status, outcome, count(*) FROM retention_tasks GROUP BY status, outcome`)
	if err != nil {
		return
	}
	for rows.Next() {
		var status string
		var outcome *string
		var n int
		if err := rows.Scan(&status, &outcome, &n); err != nil {
			rows.Close()
			return
		}
		if outcome != nil {
			byOutcome[gen.TaskOutcome(*outcome)] += n
		} else {
			byStatus[gen.TaskStatus(status)] += n
		}
	}
	rows.Close()
	if rows.Err() != nil {
		return
	}
	var overdueN int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM retention_tasks WHERE status <> 'closed' AND due_at < $1`,
		s.clk.Now()).Scan(&overdueN); err != nil {
		return
	}
	loss, ok := s.lossInWork(ctx)
	if !ok {
		return
	}

	for _, st := range openStatuses {
		ch <- prometheus.MustNewConstMetric(descTasks, prometheus.GaugeValue, float64(byStatus[st]), string(st))
	}
	ch <- prometheus.MustNewConstMetric(descTasksOverdue, prometheus.GaugeValue, float64(overdueN))
	for _, o := range outcomes {
		ch <- prometheus.MustNewConstMetric(descTasksClosed, prometheus.CounterValue, float64(byOutcome[o]), string(o))
	}
	ch <- prometheus.MustNewConstMetric(descLossInWork, prometheus.GaugeValue, float64(loss))
}

// lossInWork — сумма ожидаемой потери клиентов с открытой задачей (как фильтр очереди «в работе»); клиент
// считается, только если он есть в текущем снимке с тем же email (как customerMissing). ok=false — база недоступна.
func (s *Service) lossInWork(ctx context.Context) (int64, bool) {
	open, err := s.OpenTasks(ctx)
	if err != nil {
		return 0, false
	}
	if s.feed == nil {
		return 0, true
	}
	pf := s.feed.Portfolio()
	if pf == nil {
		return 0, true
	}
	var sum int64
	for id, ref := range open {
		c, found := findCustomer(pf, id)
		if found && strings.EqualFold(strings.TrimSpace(c.Row.Email), strings.TrimSpace(ref.Email)) {
			sum += c.Row.ExpectedLossCents
		}
	}
	return sum, true
}

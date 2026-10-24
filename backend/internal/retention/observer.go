package retention

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"example.com/app/internal/analytics"
	"example.com/app/internal/platform/api/gen"
)

// observerActor — автор наблюдений в истории: их пишет сервер, а не сотрудник.
const observerActor = "system"

// StartObserver подписывается на пересчёты и после каждого записывает наблюдения по открытым задачам.
// Подписка оформляется до возврата, поэтому событие первого пересчёта не теряется. Канал закрывается
// при остановке ctx.
func (s *Service) StartObserver(ctx context.Context) <-chan struct{} {
	stopped := make(chan struct{})
	if s.pool == nil || s.bus == nil {
		close(stopped)
		return stopped
	}
	ch, cancel := s.bus.Subscribe(analytics.TopicRecalculated)
	go func() {
		defer close(stopped)
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-ch:
				if !ok {
					return
				}
				if err := s.Observe(ctx); err != nil && ctx.Err() == nil {
					log.Printf("retention: наблюдения: %v", err)
				}
			}
		}
	}()
	return stopped
}

// Observe сравнивает открытые задачи с последним портфелем и дописывает новые наблюдения в историю.
// Закрытые задачи не трогает: окно наблюдения — от взятия до закрытия.
func (s *Service) Observe(ctx context.Context) error {
	if s.pool == nil || s.feed == nil {
		return nil
	}
	pf := s.feed.Portfolio()
	if pf == nil {
		return nil
	}
	type open struct {
		id, customerID int64
		baseline       []byte
		seenBalance    *int64
	}
	rows, err := s.pool.Query(ctx, `SELECT t.id, t.customer_id, t.baseline,
		(SELECT substr(e.obs_key, 9)::bigint FROM retention_task_events e
			WHERE e.task_id = t.id AND e.obs_key LIKE 'balance:%' ORDER BY e.id DESC LIMIT 1)
		FROM retention_tasks t WHERE t.status <> 'closed'`)
	if err != nil {
		return fmt.Errorf("открытые задачи: %w", err)
	}
	var tasks []open
	for rows.Next() {
		var o open
		if err := rows.Scan(&o.id, &o.customerID, &o.baseline, &o.seenBalance); err != nil {
			rows.Close()
			return fmt.Errorf("открытые задачи: %w", err)
		}
		tasks = append(tasks, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("открытые задачи: %w", err)
	}
	now := s.clk.Now()
	for _, o := range tasks {
		c, ok := findCustomer(pf, o.customerID)
		if !ok {
			continue
		}
		var base gen.TaskBaseline
		if err := json.Unmarshal(o.baseline, &base); err != nil {
			return fmt.Errorf("исходное состояние задачи %d: %w", o.id, err)
		}
		seen := base.Balance
		if o.seenBalance != nil {
			seen = *o.seenBalance
		}
		written := 0
		for _, ob := range observe(base, seen, c) {
			// Условие по статусу в самом запросе: задача, закрытая между чтением и записью, наблюдения не получит.
			tag, err := s.pool.Exec(ctx, `INSERT INTO retention_task_events (task_id, at, actor, kind, text, obs_key)
				SELECT id, $2, $3, 'observed', $4, $5 FROM retention_tasks WHERE id = $1 AND status <> 'closed'
				ON CONFLICT (task_id, obs_key) WHERE obs_key IS NOT NULL DO NOTHING`,
				o.id, now, observerActor, ob.Text, ob.Key)
			if err != nil {
				return fmt.Errorf("запись наблюдения по задаче %d: %w", o.id, err)
			}
			written += int(tag.RowsAffected())
		}
		if written > 0 && s.bus != nil {
			s.bus.Publish(analytics.TopicTasks, analytics.TaskEvent{Type: "observed", TaskID: o.id, CustomerID: o.customerID})
		}
	}
	return nil
}

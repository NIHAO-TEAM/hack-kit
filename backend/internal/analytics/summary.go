package analytics

import (
	"context"
	"math"

	"example.com/app/internal/platform/api/gen"
)

// GetSummary — GET /api/v1/analytics/summary.
func (a *AnalyticsAPI) GetSummary(w responseWriter, r *request) {
	o := a.latest()
	if o == nil {
		notReady(w, r)
		return
	}
	var out gen.Summary
	out.Levels.High = o.Levels["high"]
	out.Levels.Medium = o.Levels["medium"]
	out.Levels.Low = o.Levels["low"]
	out.Levels.Unscored = o.Levels["unscored"]
	out.Levels.Lost = o.Levels["lost"]
	out.ExpectedLossCents = o.ExpectedLossCents
	out.AtRiskCents = o.AtRiskCents
	out.LostCents = o.LostCents
	out.LossByFeature = lossByFeatureOut(o.LossByFeature, o.ExpectedLossCents)
	out.Version = o.Version
	if !o.AsOf.IsZero() {
		asOf := o.AsOf
		out.AsOf = &asOf
	}
	if len(o.QualityCounts) > 0 {
		q := o.QualityCounts
		out.QualityCounts = &q
	}
	out.Retention = a.retentionBlock(r.Context())
	writeJSON(w, r, out)
}

// retentionBlock считает работу с клиентами из открытых задач и текущего портфеля. Числа те же, что
// churn_retention_* в /metrics: источник один (OpenTasks). Нет источника задач, портфеля или чтение
// задач упало — nil: сводка отдаётся без блока, а не ошибкой.
func (a *AnalyticsAPI) retentionBlock(ctx context.Context) *gen.SummaryRetention {
	if a.tasks == nil {
		return nil
	}
	pf := a.portfolio()
	if pf == nil {
		return nil
	}
	open, err := a.openTasks(ctx)
	if err != nil {
		return nil
	}
	return buildRetention(open, pf)
}

func buildRetention(open map[int64]TaskRef, pf *Portfolio) *gen.SummaryRetention {
	var b gen.SummaryRetention
	for _, ref := range open {
		switch ref.Brief.Status {
		case gen.InWork:
			b.TasksInWork++
		case gen.Contacted:
			b.TasksContacted++
		case gen.Postponed:
			b.TasksPostponed++
		}
		if ref.Brief.Overdue {
			b.TasksOverdue++
		}
	}
	for _, c := range pf.Customers {
		if briefFor(open, c.Row) != nil {
			b.CustomersInWork++
			b.LossInWorkCents += c.Row.ExpectedLossCents
			continue
		}
		switch c.Row.Level {
		case gen.High:
			b.HighWithoutTask++
			b.HighWithoutTaskLossCents += c.Row.ExpectedLossCents
		case gen.Medium:
			b.MediumWithoutTask++
			b.MediumWithoutTaskLossCents += c.Row.ExpectedLossCents
		}
	}
	return &b
}

// lossByFeatureOut переводит распределение потери в ответ; доля — процент от total с двумя знаками.
func lossByFeatureOut(in []FeatureLoss, total int64) []gen.LossByFeature {
	out := make([]gen.LossByFeature, 0, len(in))
	for _, f := range in {
		share := 0.0
		if total > 0 {
			share = math.Round(float64(f.LossCents)*10000/float64(total)) / 100
		}
		out = append(out, gen.LossByFeature{FeatureId: f.ID, LossCents: f.LossCents, Customers: f.Customers, SharePercent: share})
	}
	return out
}

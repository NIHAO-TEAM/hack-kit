package analytics

import "example.com/app/internal/platform/api/gen"

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
	out.Version = o.Version
	if !o.AsOf.IsZero() {
		asOf := o.AsOf
		out.AsOf = &asOf
	}
	if len(o.QualityCounts) > 0 {
		q := o.QualityCounts
		out.QualityCounts = &q
	}
	writeJSON(w, r, out)
}

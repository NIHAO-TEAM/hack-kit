package risk

type qualityStale struct{}

func init() { RegisterCheck(qualityStale{}) }

func (qualityStale) ID() string { return "stale_status" }

// Eval: prolong_wait старше 5 дней — статус устарел, услугу давно должны были удалить.
func (qualityStale) Eval(ctx ServiceCtx) (string, bool) {
	s := ctx.Service
	return "статус устарел", s.Status == StatusProlongWait && ctx.AsOf.Sub(s.ExpiresAt) > pwaitGrace
}

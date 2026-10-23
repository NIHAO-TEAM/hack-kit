package risk

type qualitySuspended struct{}

func init() { RegisterCheck(qualitySuspended{}) }

func (qualitySuspended) ID() string { return "suspended_expired" }

// Eval: suspended с истёкшим сроком — простой неизвестной длительности.
func (qualitySuspended) Eval(ctx ServiceCtx) (string, bool) {
	s := ctx.Service
	return "выключена, срок истёк", s.Status == StatusSuspended && s.ExpiresAt.Before(ctx.AsOf)
}
func (qualitySuspended) Subject() string {
	return "в статусе «выключена» с истёкшим сроком"
}

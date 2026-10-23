package risk

type qualityBlocked struct{}

func init() { RegisterCheck(qualityBlocked{}) }

func (qualityBlocked) ID() string { return "blocked_future" }

// Eval: blocked со сроком в будущем — противоречие данных.
func (qualityBlocked) Eval(ctx ServiceCtx) (string, bool) {
	s := ctx.Service
	return "заблокирована, срок в будущем", s.Status == StatusBlocked && s.ExpiresAt.After(ctx.AsOf)
}
func (qualityBlocked) Subject() string {
	return "в статусе «заблокирована» со сроком в будущем"
}

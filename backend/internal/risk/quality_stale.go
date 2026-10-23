package risk

type qualityStale struct{}

func init() { RegisterCheck(qualityStale{}) }

func (qualityStale) ID() string { return "stale_status" }

// Eval: prolong_wait старше 5 дней — ошибка данных (эксперт aéza: ошибка генератора), на скор не влияет.
func (qualityStale) Eval(ctx ServiceCtx) (string, bool) {
	s := ctx.Service
	return "ошибка данных: в ожидании оплаты дольше 5 дней", s.Status == StatusProlongWait && ctx.AsOf.Sub(s.ExpiresAt) > pwaitGrace
}

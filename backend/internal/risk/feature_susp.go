package risk

type susp struct{}

func init() { Register(susp{}) }

func (susp) ID() string             { return "susp" }
func (susp) DefaultWeight() float64 { return 0.35 }

// Eval: услуга выключена клиентом, но оплачивается (правило 5).
func (susp) Eval(ctx ServiceCtx) (float64, string, bool) {
	if ctx.Service.Status != StatusSuspended {
		return 0, "", false
	}
	return 1, "Услуга выключена клиентом, но продолжает оплачиваться", true
}

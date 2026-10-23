package risk

type autoProlong struct{}

func init() { RegisterAction(autoProlong{}) }

func (autoProlong) ID() string { return "enable_autoprolong" }

// Applies: у живой услуги автопродление выключено и срок меньше 7 дней (noauto > 0, правило 5).
func (autoProlong) Applies(ctx CustomerCtx) bool {
	for _, sr := range ctx.Customer.Services {
		for _, h := range sr.Features {
			if h.ID == "noauto" && h.F > 0 {
				return true
			}
		}
	}
	return false
}

func (autoProlong) Title(CustomerCtx) string { return "Включить автопродление" }

// Apply включает автопродление у всех живых услуг клиента, как в оракуле.
func (autoProlong) Apply(s Snapshot, accountID int64) Snapshot {
	out := s
	out.Services = append([]Service(nil), s.Services...)
	for i, sv := range out.Services {
		if sv.OwnerID == accountID && sv.IsLive() {
			out.Services[i].AutoProlong = true
		}
	}
	return out
}

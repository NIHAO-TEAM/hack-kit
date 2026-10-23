package risk

type toBilling struct{}

func init() { RegisterAction(toBilling{}) }

func (toBilling) ID() string { return "to_billing" }

// Applies: автопродление включено и денег хватает, а услуга не продлена — это сбой списания, не отток.
func (toBilling) Applies(ctx CustomerCtx) bool {
	if ctx.Funds.Have < ctx.Funds.Need {
		return false
	}
	for _, sv := range ctx.Services {
		if !sv.AutoProlong {
			continue
		}
		switch sv.Status {
		case StatusProlongWait, StatusDeleted, StatusBlocked:
			return true
		}
	}
	return false
}

func (toBilling) Title(CustomerCtx) string { return "К биллингу: сбой списания" }

// Apply не меняет данные: эффект 0.
func (toBilling) Apply(s Snapshot, _ int64) Snapshot { return s }

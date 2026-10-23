package source

import (
	"strconv"
	"time"

	"example.com/app/internal/audit"
)

func findService(s *Snapshot, id int64) Service {
	for _, v := range s.Services {
		if v.ID == id {
			return v
		}
	}
	return Service{}
}

func findAccount(s *Snapshot, id int64) Account {
	for _, v := range s.Accounts {
		if v.ID == id {
			return v
		}
	}
	return Account{}
}

// serviceChanges — что поменялось в услуге. Названий и владельцев в журнал не кладём.
func serviceChanges(before, after Service) []audit.Change {
	var out []audit.Change
	if before.Status != after.Status {
		out = append(out, audit.Change{Field: "status", From: before.Status, To: after.Status})
	}
	if before.AutoProlong != after.AutoProlong {
		out = append(out, audit.Change{Field: "autoProlong", From: strconv.FormatBool(before.AutoProlong), To: strconv.FormatBool(after.AutoProlong)})
	}
	if !before.ExpiresAt.Equal(after.ExpiresAt) {
		out = append(out, audit.Change{Field: "expiresAt", From: before.ExpiresAt.UTC().Format(time.RFC3339), To: after.ExpiresAt.UTC().Format(time.RFC3339)})
	}
	return out
}

// accountChanges — что поменялось в счёте: только суммы в центах, email клиента в журнал не попадает.
func accountChanges(before, after Account) []audit.Change {
	var out []audit.Change
	if before.Balance != after.Balance {
		out = append(out, audit.Change{Field: "balance", From: strconv.FormatInt(before.Balance, 10), To: strconv.FormatInt(after.Balance, 10)})
	}
	if before.BonusBalance != after.BonusBalance {
		out = append(out, audit.Change{Field: "bonusBalance", From: strconv.FormatInt(before.BonusBalance, 10), To: strconv.FormatInt(after.BonusBalance, 10)})
	}
	return out
}

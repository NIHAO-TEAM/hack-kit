package source

import (
	"errors"
	"testing"
	"time"

	"example.com/app/internal/platform/api/gen"
)

func i64(v int64) *int64 { return &v }

func patchSnap() Snapshot {
	at := time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC)
	return Snapshot{
		AsOf:     &at,
		Accounts: []Account{{ID: 9, Email: "a@x.test", Balance: 100, BonusState: "locked"}},
		Services: []Service{{ID: 5, Name: "vps-1", OwnerID: 9, PaymentTerm: "month", ExpiresAt: at, CreatedAt: at, Status: "active", Price: 500}},
	}
}

func TestPatchAccount(t *testing.T) {
	s := patchSnap()
	got, err := patchAccount(&s, 9, gen.AccountPatch{Balance: i64(100 + 52984), BonusBalance: i64(7)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Balance != 53084 || got.BonusBalance != 7 || s.Accounts[0].Balance != 53084 {
		t.Errorf("счёт после правки %+v", got)
	}
}

func TestPatchAccountKeepsUnsetFields(t *testing.T) {
	s := patchSnap()
	got, err := patchAccount(&s, 9, gen.AccountPatch{BonusBalance: i64(3)})
	if err != nil || got.Balance != 100 {
		t.Errorf("баланс не должен меняться: %+v %v", got, err)
	}
}

func TestPatchService(t *testing.T) {
	s := patchSnap()
	on, status := true, gen.ServiceStatus("prolong_wait")
	exp := time.Date(2026, 11, 1, 0, 0, 0, 0, time.FixedZone("x", 3600))
	got, err := patchService(&s, 5, gen.ServicePatch{AutoProlong: &on, Status: &status, ExpiresAt: &exp})
	if err != nil {
		t.Fatal(err)
	}
	if !got.AutoProlong || got.Status != "prolong_wait" || !got.ExpiresAt.Equal(exp) || got.ExpiresAt.Location() != time.UTC {
		t.Errorf("услуга после правки %+v", got)
	}
}

func TestPatchErrors(t *testing.T) {
	s := patchSnap()
	var nf notFound
	if _, err := patchService(&s, 404, gen.ServicePatch{}); !errors.As(err, &nf) {
		t.Errorf("неизвестная услуга: %v", err)
	}
	if _, err := patchAccount(&s, 404, gen.AccountPatch{}); !errors.As(err, &nf) {
		t.Errorf("неизвестный счёт: %v", err)
	}
	var br badRequest
	bad := gen.ServiceStatus("zombie")
	if _, err := patchService(&s, 5, gen.ServicePatch{Status: &bad}); !errors.As(err, &br) {
		t.Errorf("неизвестный статус должен быть 400: %v", err)
	}
	if _, err := patchAccount(&s, 9, gen.AccountPatch{Balance: i64(-1)}); !errors.As(err, &br) {
		t.Errorf("отрицательный баланс должен быть 400: %v", err)
	}
}

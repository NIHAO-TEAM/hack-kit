//go:build integration

package app_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"example.com/app/internal/auth"
	"example.com/app/internal/platform/api/gen"
)

// Клиенты на данных организаторов через настоящий API: сессия, пересчёт, выдача.
func TestCustomersAPI(t *testing.T) {
	r := newRig(t, true)
	r.StartRecalc(t)
	c := r.As(t, auth.RoleMarketer)
	ctx := context.Background()

	var page *gen.CustomerPage
	deadline := time.Now().Add(10 * time.Second)
	for page == nil {
		res, err := c.ListCustomersWithResponse(ctx, &gen.ListCustomersParams{})
		if err != nil {
			t.Fatal(err)
		}
		if res.JSON200 != nil {
			page = res.JSON200
		} else if time.Now().After(deadline) {
			t.Fatalf("клиенты не появились: %d %s", res.StatusCode(), res.Body)
		} else {
			time.Sleep(50 * time.Millisecond)
		}
	}
	if page.Total != 10 || page.Items[0].Id != 9 || page.Items[0].ExpectedLossCents != 345927 || page.Items[1].Id != 1 {
		t.Errorf("рейтинг: total=%d, первая строка %+v", page.Total, page.Items[0])
	}

	d, err := c.GetCustomerWithResponse(ctx, 9)
	if err != nil || d.JSON200 == nil {
		t.Fatalf("№9: %v %d", err, d.StatusCode())
	}
	if len(d.JSON200.Actions) == 0 || d.JSON200.Actions[0].EffectCents != -106748 {
		t.Errorf("действия №9: %+v", d.JSON200.Actions)
	}
	if res, _ := c.GetCustomerWithResponse(ctx, 424242); res.StatusCode() != http.StatusNotFound {
		t.Errorf("неизвестный ID: %d", res.StatusCode())
	}
	if res, _ := r.Anonymous(t).ListCustomersWithResponse(ctx, &gen.ListCustomersParams{}); res.StatusCode() != http.StatusUnauthorized {
		t.Errorf("без сессии: %d", res.StatusCode())
	}
}

// Правка баланса №9 на 529,84 € через PATCH: потеря падает на 1 067,48 €, очередь пересчитана.
func TestPatchAccountRecalculatesLoss(t *testing.T) {
	r := newRig(t, true)
	r.StartRecalc(t)
	ctx := context.Background()
	marketer := r.As(t, auth.RoleMarketer)

	loss := func(wantLoss int64) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for {
			d, err := marketer.GetCustomerWithResponse(ctx, 9)
			if err == nil && d.JSON200 != nil && d.JSON200.Customer.ExpectedLossCents == wantLoss {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("потеря №9 не стала %d", wantLoss)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	loss(345927)

	if res, _ := marketer.PatchAccountWithResponse(ctx, 9, gen.AccountPatch{}); res.StatusCode() != http.StatusForbidden {
		t.Errorf("маркетолог правит данные: %d", res.StatusCode())
	}
	analyst := r.As(t, auth.RoleAnalyst)
	balance := int64(91506 + 52984) // баланс №9 в сиде организаторов + 529,84 €
	res, err := analyst.PatchAccountWithResponse(ctx, 9, gen.AccountPatch{Balance: &balance})
	if err != nil || res.JSON200 == nil {
		t.Fatalf("PATCH: %v %d %s", err, res.StatusCode(), res.Body)
	}
	loss(345927 - 106748)
}

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

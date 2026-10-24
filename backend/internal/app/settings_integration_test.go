//go:build integration

package app_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"example.com/app/internal/app"
	"example.com/app/internal/model"
	"example.com/app/internal/platform/api/gen"
)

func patch(t *testing.T, c *gen.ClientWithResponses, dryRun bool, body string) (int, []byte) {
	t.Helper()
	res, err := c.PatchModelSettingsWithBody(context.Background(), &gen.PatchModelSettingsParams{DryRun: &dryRun}, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, b
}

func TestSettingsAPI(t *testing.T) {
	r := newRig(t, true)
	ctx := context.Background()
	analyst := r.As(t, "analyst")
	ch, cancel := r.Bus.Subscribe("model.changed")
	defer cancel()

	get := func() *gen.ModelSettings {
		res, err := analyst.GetModelSettingsWithResponse(ctx)
		if err != nil || res.JSON200 == nil {
			t.Fatalf("GET: %v %d %s", err, res.StatusCode(), res.Body)
		}
		return res.JSON200
	}

	if s := get(); s.Version != 0 || s.Weights["noauto"] != 0.55 || s.UpdatedBy != nil {
		t.Fatalf("умолчания: %+v", s)
	}

	// Неверные значения — 422 с полями, версия прежняя.
	for _, body := range []string{
		`{"weights":{"noauto":1.2}}`,
		`{"thresholds":{"high":40}}`,
		`{"horizonDays":400}`,
		`{"weights":{"zzz":0.5}}`,
		`{"priceBasis":"x"}`,
	} {
		code, resp := patch(t, analyst, false, body)
		if code != http.StatusUnprocessableEntity || !strings.Contains(string(resp), `"errors"`) {
			t.Errorf("%s: %d %s", body, code, resp)
		}
	}
	if code, _ := patch(t, analyst, false, `{"nope":1}`); code != http.StatusBadRequest {
		t.Errorf("неизвестное поле: %d", code)
	}
	if get().Version != 0 {
		t.Fatal("версия изменилась после 422")
	}

	// dryRun: ничего не сохраняет.
	code, resp := patch(t, analyst, true, `{"weights":{"noauto":0.85}}`)
	if code != http.StatusOK || !strings.Contains(string(resp), "expectedLossDeltaCents") {
		t.Fatalf("dryRun: %d %s", code, resp)
	}
	if !strings.Contains(string(resp), "->high") || !strings.Contains(string(resp), "20069") {
		t.Errorf("dryRun: ждали смену уровня на высокий и потерю +20069: %s", resp)
	}
	var dr gen.DryRunResult
	if err := json.Unmarshal(resp, &dr); err != nil {
		t.Fatal(err)
	}
	changed := 0
	for _, n := range dr.LevelChanges {
		changed += n
	}
	if dr.CustomersTotal == nil || *dr.CustomersTotal != changed || dr.Customers == nil || len(*dr.Customers) != changed {
		t.Errorf("dryRun: customers/customersTotal не совпали со сменами уровня (%d): %s", changed, resp)
	}
	if get().Version != 0 {
		t.Fatal("dryRun сохранил настройки")
	}

	// Сохранение: версия 1, событие, автор.
	code, resp = patch(t, analyst, false, `{"weights":{"noauto":0.85}}`)
	if code != http.StatusOK {
		t.Fatalf("PATCH: %d %s", code, resp)
	}
	select {
	case ev := <-ch:
		if ev.Payload != int64(1) {
			t.Errorf("payload = %v", ev.Payload)
		}
	default:
		t.Error("нет события model.changed")
	}
	s := get()
	if s.Version != 1 || s.Weights["noauto"] != 0.85 || s.Weights["pwait"] != 0.8 || s.UpdatedBy == nil || *s.UpdatedBy != "analyst" {
		t.Errorf("после PATCH: %+v", s)
	}

	// После «перезапуска» (новое хранилище на той же базе) настройки на месте, пересчёт их видит.
	rec, err := model.NewStore(r.Pool, r.Bus, app.ValidateSettings).Latest(ctx)
	if err != nil || rec.Version != 1 || !strings.Contains(string(rec.Body), `"noauto":0.85`) {
		t.Errorf("после перезапуска: %+v, %v", rec, err)
	}
}

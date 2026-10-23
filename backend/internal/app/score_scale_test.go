package app_test

import (
	"bytes"
	"testing"
	"time"

	"example.com/app/internal/app"
	"example.com/app/internal/risk"
	"example.com/app/internal/source"
	"example.com/app/tools/snapgen"
)

// Пересчёт не квадратичен по числу клиентов: действия считаются на копии одного клиента,
// а не всего снимка. До исправления 3 000 аккаунтов считались около 7 с (без -race), 10 000 — полторы минуты.
func TestScoreScalesWithAccounts(t *testing.T) {
	body, err := snapgen.Generate(snapgen.Options{Accounts: 3000, Seed: 11})
	if err != nil {
		t.Fatal(err)
	}
	s, errs, err := source.ParseSnapshot(bytes.NewReader(body))
	if err != nil || len(errs) > 0 {
		t.Fatalf("разбор: %v %v", err, errs)
	}
	rs := app.ToRiskSnapshot(*s)
	start := time.Now()
	res := risk.Score(rs, risk.Defaults(), snapgen.AsOf)
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("3 000 аккаунтов считались %v, ждали быстрее 5 с", took)
	}
	if len(res.Customers) != 3000 {
		t.Errorf("клиентов %d", len(res.Customers))
	}
}

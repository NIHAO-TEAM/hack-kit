package app_test

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"testing"

	"example.com/app/internal/app"
	"example.com/app/internal/risk"
	"example.com/app/internal/source"
	"example.com/app/tools/snapgen"
)

// checkScored проверяет, что результат модели на принятом снимке согласован: нет NaN и Inf,
// потерь меньше нуля, скора вне [0, 100] (проценты), а сводка потерь равна сумме по клиентам («под угрозой» — только high и medium, не больше суммы).
func checkScored(res risk.Result) error {
	var loss, exposure, lost int64
	for _, c := range res.Customers {
		if c.Score != nil && (math.IsNaN(*c.Score) || *c.Score < 0 || *c.Score > 100) {
			return fmt.Errorf("клиент %d: скор %v вне [0, 100]", c.ID, *c.Score)
		}
		if c.ExpectedLossCents < 0 || c.ExposureCents < 0 || c.LostCents < 0 {
			return fmt.Errorf("клиент %d: отрицательная сумма %d/%d/%d", c.ID, c.ExpectedLossCents, c.ExposureCents, c.LostCents)
		}
		loss += c.ExpectedLossCents
		exposure += c.ExposureCents
		lost += c.LostCents
	}
	s := res.Summary
	if s.ExpectedLossCents != loss || s.ExposureCents > exposure || s.ExposureCents < 0 || s.LostCents != lost {
		return fmt.Errorf("сводка %d/%d/%d не согласована с суммами по клиентам %d/%d/%d",
			s.ExpectedLossCents, s.ExposureCents, s.LostCents, loss, exposure, lost)
	}
	return nil
}

// FuzzScoreAcceptedSnapshot: любой принятый приёмом снимок считается моделью без паники,
// с согласованными числами (сквозная проверка после FuzzParseSnapshot в модуле source).
func FuzzScoreAcceptedSnapshot(f *testing.F) {
	if b, err := os.ReadFile("../../../cases/analizator/data.json"); err == nil {
		f.Add(b)
	}
	for _, edge := range append([]string{snapgen.EdgeNone}, snapgen.Edges...) {
		b, err := snapgen.Generate(snapgen.Options{Accounts: 5, Seed: 1, Edge: edge})
		if err != nil {
			f.Fatal(err)
		}
		f.Add(b)
	}
	for _, s := range []string{
		``, `{}`, `[]`, `null`, `{"accounts":null,"services":null,"transactions":null}`,
		`{"accounts":[{"id":1}],"services":[],"transactions":[]}`,
		`{"accounts":[],"services":[],"transactions":[]}{"x":1}`,
		`{"accounts":[1],"services":[],"transactions":[]}`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		s, errs, err := source.ParseSnapshot(bytes.NewReader(body))
		if err != nil {
			return
		}
		errs = append(errs, source.Validate(s)...)
		if len(errs) > 0 {
			return
		}
		rs := app.ToRiskSnapshot(*s)
		at := source.ModelTime(s, snapgen.AsOf)
		if err := checkScored(risk.Score(rs, risk.Defaults(), at)); err != nil {
			t.Fatal(err)
		}
	})
}

package source_test

import (
	"bytes"
	"os"
	"testing"

	"example.com/app/internal/source"
	"example.com/app/tools/snapgen"
)

// FuzzParseSnapshot: любой вход — либо ошибка формата (400), либо ошибки полей (422), либо
// принятый снимок; разбор и проверка не паникуют, у принятого снимка нет сумм вне границ.
func FuzzParseSnapshot(f *testing.F) {
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
		if errs = append(errs, source.Validate(s)...); len(errs) > 0 {
			return
		}
		const limit = 1_000_000_000
		for _, a := range s.Accounts {
			for _, v := range []int64{a.Balance, a.WithdrawBalance, a.TotalReplenished, a.BonusBalance} {
				if v < 0 || v > limit {
					t.Fatalf("аккаунт %d принят с суммой %d", a.ID, v)
				}
			}
		}
		for _, v := range s.Services {
			if v.Price < 0 || v.Price > limit {
				t.Fatalf("услуга %d принята с ценой %d", v.ID, v.Price)
			}
			if v.ExpiresAt.Before(v.CreatedAt) {
				t.Fatalf("услуга %d принята с expiresAt раньше createdAt", v.ID)
			}
		}
		for _, tx := range s.Transactions {
			if tx.Amount < 0 || tx.Amount > 1000 {
				t.Fatalf("платёж %d принят с суммой %d", tx.ID, tx.Amount)
			}
		}
	})
}

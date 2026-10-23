package snapgen_test

import (
	"bytes"
	"testing"

	"example.com/app/internal/source"
	"example.com/app/tools/snapgen"
)

func TestDeterministic(t *testing.T) {
	for _, edge := range append([]string{snapgen.EdgeNone}, snapgen.Edges...) {
		o := snapgen.Options{Accounts: 40, Seed: 7, Edge: edge}
		a, err := snapgen.Generate(o)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := snapgen.Generate(o)
		if !bytes.Equal(a, b) {
			t.Errorf("%q: два запуска с одним seed дали разные файлы", edge)
		}
		o.Seed = 8
		if c, _ := snapgen.Generate(o); edge != snapgen.EdgeEmpty && bytes.Equal(a, c) {
			t.Errorf("%q: другой seed дал тот же файл", edge)
		}
	}
}

// Всё сгенерированное проходит разбор и проверки приёма: файл годится для PUT /source/snapshot.
func TestGeneratedIsValid(t *testing.T) {
	for _, edge := range append([]string{snapgen.EdgeNone}, snapgen.Edges...) {
		body, err := snapgen.Generate(snapgen.Options{Accounts: 200, Seed: 3, Edge: edge})
		if err != nil {
			t.Fatal(err)
		}
		s, errs, err := source.ParseSnapshot(bytes.NewReader(body))
		if err != nil || len(errs) > 0 {
			t.Fatalf("%q: разбор: %v %v", edge, err, errs)
		}
		if errs := source.Validate(s); len(errs) > 0 {
			t.Fatalf("%q: проверка: %v", edge, errs)
		}
	}
}

func TestUnknownEdge(t *testing.T) {
	if _, err := snapgen.Generate(snapgen.Options{Edge: "nope"}); err == nil {
		t.Fatal("ждали ошибку")
	}
}

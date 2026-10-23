package risk

import "testing"

type stubFeature struct {
	id string
	w  float64
}

func (s stubFeature) ID() string             { return s.id }
func (s stubFeature) DefaultWeight() float64 { return s.w }
func (stubFeature) Eval(ServiceCtx) (float64, string, bool) {
	return 0.5, "тест", true
}

func TestRegistryFeatureAppearsInDefaults(t *testing.T) {
	r := newRegistry[Feature]()
	r.register("признак", stubFeature{"zeta", 0.3})
	r.register("признак", stubFeature{"alpha", 0.2})
	w := defaultsFrom(r).Weights
	if w["zeta"] != 0.3 || w["alpha"] != 0.2 || len(w) != 2 {
		t.Fatalf("веса: %v", w)
	}
	all := r.all()
	if all[0].ID() != "alpha" || all[1].ID() != "zeta" {
		t.Fatalf("порядок не по ID: %v", all)
	}
}

func TestRegistryDuplicatePanics(t *testing.T) {
	r := newRegistry[Feature]()
	r.register("признак", stubFeature{"a", 0.1})
	defer func() {
		if recover() == nil {
			t.Fatal("повторная регистрация должна паниковать")
		}
	}()
	r.register("признак", stubFeature{"a", 0.1})
}

func TestBuiltinFeaturesRegistered(t *testing.T) {
	w := Defaults().Weights
	for _, id := range []string{"pwait", "noauto"} {
		if _, ok := w[id]; !ok {
			t.Errorf("признак %s не зарегистрирован", id)
		}
	}
}

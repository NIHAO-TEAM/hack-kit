package backtest

import (
	"math"
	"testing"
	"time"

	"example.com/app/internal/source"
	"example.com/app/tools/snapgen"
)

func t0() time.Time { return time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC) }

// Два клиента без поведения: у первого услуга без автопродления кончается через сутки и удаляется
// (сутки + 5 дней в prolong_wait); у второго автопродление и деньги — услуга живёт.
func tinySnapshot() source.Snapshot {
	at := t0()
	return source.Snapshot{
		AsOf: &at,
		Accounts: []source.Account{
			{ID: 1, Email: "a@x", Balance: 0, TotalReplenished: 5000, BonusState: "none"},
			{ID: 2, Email: "b@x", Balance: 1000000, TotalReplenished: 1000000, BonusState: "none"},
		},
		Services: []source.Service{
			{ID: 10, Name: "vds", OwnerID: 1, PaymentTerm: "month", AutoProlong: false,
				CreatedAt: at.AddDate(0, -3, 0), ExpiresAt: at.Add(24 * time.Hour), Status: "active", Price: 1000},
			{ID: 20, Name: "vds", OwnerID: 2, PaymentTerm: "month", AutoProlong: true,
				CreatedAt: at.AddDate(0, -3, 0), ExpiresAt: at.Add(24 * time.Hour), Status: "active", Price: 1000},
		},
	}
}

func TestKnownOutcome(t *testing.T) {
	p, err := Prepare(tinySnapshot())
	if err != nil {
		t.Fatal(err)
	}
	lost := p.outcomes(Behavior{}, 1)
	got := map[int64]bool{}
	for i, sv := range p.svcs {
		got[sv.id] = lost[i]
	}
	if !got[10] || got[20] {
		t.Fatalf("исходы услуг: %v, ждали 10 — ушла, 20 — осталась", got)
	}
	r := p.Run(Behavior{}, 1)
	if r.LostServices != 1 || r.Services != 2 || r.LostCents <= 0 {
		t.Fatalf("итог: %+v", r)
	}
	// единственная потеря у клиента 1: модель и деньги ставят его наверх
	if m := r.ByMethod[Model]; m.Top20 != 1 || m.ROC != 1 {
		t.Fatalf("модель на известном исходе: %+v", m)
	}
}

func TestDeterministic(t *testing.T) {
	body, err := snapgen.Generate(snapgen.Options{Accounts: 300, Seed: 7})
	if err != nil {
		t.Fatal(err)
	}
	s := parse(t, body)
	b, err := DefaultBehavior()
	if err != nil {
		t.Fatal(err)
	}
	p, err := Prepare(s)
	if err != nil {
		t.Fatal(err)
	}
	a, c := p.Run(b, 5), p.Run(b, 5)
	if a.LostCents != c.LostCents || a.ByMethod[Model] != c.ByMethod[Model] || a.ByMethod[Random] != c.ByMethod[Random] {
		t.Fatalf("одно зерно — разные результаты:\n%+v\n%+v", a, c)
	}
	if d := p.Run(b, 6); d.LostCents == a.LostCents && d.ByMethod[Model] == a.ByMethod[Model] {
		t.Fatal("другое зерно дало тот же результат: поведение не использует зерно")
	}
}

func TestBehaviorOnlyObservableFields(t *testing.T) {
	// поведение — чистая функция снимка и rng: счёт модели его не видит (скор считается один раз до прокрутки)
	b, _ := DefaultBehavior()
	for _, p := range Params {
		hi := b.Scaled(p.Name, 100)
		if *p.ptr(&hi) > 1 {
			t.Fatalf("%s: вероятность выше 1", p.Name)
		}
	}
	if z := b.Scaled("", 0); z != (Behavior{TopUpShareMin: b.TopUpShareMin, TopUpShareMax: b.TopUpShareMax}) {
		t.Fatalf("нулевой множитель должен обнулять все вероятности: %+v", z)
	}
}

func TestRules(t *testing.T) {
	p, err := Prepare(tinySnapshot())
	if err != nil {
		t.Fatal(err)
	}
	r := p.Run(Behavior{}, 1)
	for _, m := range []string{NoAutoprolng, LowFunds, Price} {
		if x := r.ByMethod[m]; x.Top20 != 1 && m != Price {
			t.Errorf("%s: правило не ставит ушедшего наверх: %+v", m, x)
		}
	}
	// prolong_wait на T никого нет: все ключи равны, порядок по id, ROC = 0,5
	if x := r.ByMethod[ProlongWait]; x.ROC != 0.5 {
		t.Errorf("prolong_wait без ключей: ROC %v, ждали 0,5", x.ROC)
	}
	if x := r.ByMethod[Random]; x.ROC < 0.2 || x.ROC > 0.8 {
		t.Errorf("random: ROC %v вне разумного", x.ROC)
	}
}

func TestRobustnessCount(t *testing.T) {
	p, _ := Prepare(tinySnapshot())
	got := p.Robustness(Behavior{}, 1)
	if want := (len(Params) + 1) * 2; len(got) != want {
		t.Fatalf("сдвигов %d, ждали %d", len(got), want)
	}
}

func TestPrepareNeedsAsOf(t *testing.T) {
	if _, err := Prepare(source.Snapshot{}); err == nil {
		t.Fatal("снимок без asOf должен давать ошибку")
	}
}

func parse(t *testing.T, body []byte) source.Snapshot {
	t.Helper()
	s, errs, err := source.ParseSnapshot(bytesReader(body))
	if err != nil || len(errs) > 0 {
		t.Fatal(err, errs)
	}
	return *s
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

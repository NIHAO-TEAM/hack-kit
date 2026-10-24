package backtest

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"math"
	"math/rand/v2"
	"os"

	"example.com/app/internal/source"
)

//go:embed behavior.json
var defaultBehavior []byte

// Behavior — допущения о поведении клиента: вероятности за сутки и доли пополнения.
// Это допущения команды, а не данные aéza; они не берутся из весов модели и не зависят от скора.
// Вероятности зависят только от наблюдаемых полей: баланса, статуса и автопродления.
type Behavior struct {
	// TopUpLow — пополнение, если на балансе меньше суммы цен живых услуг; TopUpIdle — если хватает.
	TopUpLow  float64 `json:"topUpLow"`
	TopUpIdle float64 `json:"topUpIdle"`
	// Сумма пополнения — доля от totalReplenished, равномерно от Min до Max (не меньше суммы цен услуг).
	TopUpShareMin float64 `json:"topUpShareMin"`
	TopUpShareMax float64 `json:"topUpShareMax"`
	// EnableAuto и DisableAuto — включить автопродление активной услуги и выключить его.
	EnableAuto  float64 `json:"enableAuto"`
	DisableAuto float64 `json:"disableAuto"`
	// Suspend — клиент выключает активную услугу; Resume — включает выключенную.
	Suspend float64 `json:"suspend"`
	Resume  float64 `json:"resume"`
	// SelfDelete — клиент сам удаляет живую услугу; SelfDeleteWaiting — то же для услуги в prolong_wait.
	SelfDelete        float64 `json:"selfDelete"`
	SelfDeleteWaiting float64 `json:"selfDeleteWaiting"`
}

// DefaultBehavior читает вшитый behavior.json.
func DefaultBehavior() (Behavior, error) {
	var b Behavior
	if err := json.Unmarshal(defaultBehavior, &b); err != nil {
		return b, fmt.Errorf("behavior.json: %w", err)
	}
	return b, nil
}

// Param — вероятность, которую сдвигает проверка устойчивости.
type Param struct {
	Name string
	ptr  func(*Behavior) *float64
}

// Params — вероятности поведения в порядке отчёта (доли суммы пополнения не сдвигаются).
var Params = []Param{
	{"topUpLow", func(b *Behavior) *float64 { return &b.TopUpLow }},
	{"topUpIdle", func(b *Behavior) *float64 { return &b.TopUpIdle }},
	{"enableAuto", func(b *Behavior) *float64 { return &b.EnableAuto }},
	{"disableAuto", func(b *Behavior) *float64 { return &b.DisableAuto }},
	{"suspend", func(b *Behavior) *float64 { return &b.Suspend }},
	{"resume", func(b *Behavior) *float64 { return &b.Resume }},
	{"selfDelete", func(b *Behavior) *float64 { return &b.SelfDelete }},
	{"selfDeleteWaiting", func(b *Behavior) *float64 { return &b.SelfDeleteWaiting }},
}

// Scaled возвращает копию, где вероятность name умножена на k; name пустой — все вероятности.
func (b Behavior) Scaled(name string, k float64) Behavior {
	for _, p := range Params {
		if name == "" || p.Name == name {
			v := p.ptr(&b)
			*v = math.Min(1, *v*k)
		}
	}
	return b
}

// behave делает один сутки поведения клиентов; s меняется на месте. Порядок обхода фиксирован,
// поэтому один rng — один результат.
func behave(s *source.Snapshot, b Behavior, rng *rand.Rand) {
	idx := make(map[int64][]int, len(s.Accounts))
	for i, v := range s.Services {
		idx[v.OwnerID] = append(idx[v.OwnerID], i)
	}
	for ai := range s.Accounts {
		a := &s.Accounts[ai]
		var need int64
		for _, i := range idx[a.ID] {
			if live(s.Services[i].Status) {
				need += s.Services[i].Price
			}
		}
		if need > 0 {
			p := b.TopUpIdle
			if a.Balance < need {
				p = b.TopUpLow
			}
			if rng.Float64() < p {
				share := b.TopUpShareMin + rng.Float64()*(b.TopUpShareMax-b.TopUpShareMin)
				amount := int64(math.Round(float64(a.TotalReplenished) * share))
				if amount < need {
					amount = need
				}
				a.Balance += amount
				a.TotalReplenished += amount
			}
		}
		for _, i := range idx[a.ID] {
			v := &s.Services[i]
			if !live(v.Status) {
				continue
			}
			if rng.Float64() < b.selfDeleteProb(v.Status) {
				v.Status = "deleted"
				continue
			}
			switch v.Status {
			case "active":
				if v.AutoProlong && rng.Float64() < b.DisableAuto {
					v.AutoProlong = false
				} else if !v.AutoProlong && rng.Float64() < b.EnableAuto {
					v.AutoProlong = true
				}
				if rng.Float64() < b.Suspend {
					v.Status = "suspended"
				}
			case "suspended":
				if rng.Float64() < b.Resume {
					v.Status = "active"
				}
			}
		}
	}
}

func (b Behavior) selfDeleteProb(status string) float64 {
	if status == "prolong_wait" {
		return b.SelfDeleteWaiting
	}
	return b.SelfDelete
}

func live(status string) bool {
	return status == "active" || status == "suspended" || status == "prolong_wait"
}

// LoadBehavior читает допущения поведения из файла.
func LoadBehavior(path string) (Behavior, error) {
	var b Behavior
	raw, err := os.ReadFile(path)
	if err != nil {
		return b, err
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return b, fmt.Errorf("%s: %w", path, err)
	}
	return b, nil
}

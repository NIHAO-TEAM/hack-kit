package risk

import (
	"fmt"
	"sort"
	"time"
)

// FundsCtx — «нужно» и «есть» по аккаунту за окно запаса средств (центы).
// Считается один раз на аккаунт, признак funds только читает.
type FundsCtx struct {
	Need int64
	Have int64
}

// ServiceCtx — всё, что нужно признаку или проверке: услуга, её аккаунт, транзакции услуги, asOf.
type ServiceCtx struct {
	AsOf         time.Time
	Account      Account
	Service      Service
	Transactions []Transaction
	Funds        FundsCtx
}

// Feature — признак риска. Eval возвращает f ∈ [0, 1], текст причины и ok = «признак применим к услуге».
type Feature interface {
	ID() string
	DefaultWeight() float64
	Eval(ctx ServiceCtx) (f float64, reason string, ok bool)
}

// Check — проверка качества данных услуги (наполняет risk-features).
type Check interface {
	ID() string
	Eval(ctx ServiceCtx) (msg string, hit bool)
}

// CustomerCtx — клиент для действий: аккаунт, его результат с признаками и «нужно/есть».
type CustomerCtx struct {
	Account  Account
	Customer Customer
	Funds    FundsCtx
	Services []Service // все услуги аккаунта, в том числе не живые
}

// Action — действие с ценой (наполняет money-actions). Apply не меняет переданный снимок.
type Action interface {
	ID() string
	Applies(ctx CustomerCtx) bool
	Apply(s Snapshot, accountID int64) Snapshot
	Title(ctx CustomerCtx) string
}

// settingsAction — действие, которому для Apply нужны настройки расчёта (например, основа цены).
type settingsAction interface {
	Action
	ApplyWith(s Snapshot, accountID int64, st Settings) Snapshot
}

type registry[T interface{ ID() string }] struct{ items map[string]T }

func newRegistry[T interface{ ID() string }]() *registry[T] {
	return &registry[T]{items: map[string]T{}}
}

func (r *registry[T]) register(kind string, v T) {
	if v.ID() == "" {
		panic("risk: " + kind + " без идентификатора")
	}
	if _, dup := r.items[v.ID()]; dup {
		panic(fmt.Sprintf("risk: %s %q уже зарегистрирован", kind, v.ID()))
	}
	r.items[v.ID()] = v
}

// all — элементы в порядке ID: результат не зависит от порядка init().
func (r *registry[T]) all() []T {
	out := make([]T, 0, len(r.items))
	for _, v := range r.items {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

type featureRegistry = registry[Feature]

var (
	features = newRegistry[Feature]()
	checks   = newRegistry[Check]()
	actions  = newRegistry[Action]()
)

// Register добавляет признак; вызывается из init() файла признака.
func Register(f Feature) { features.register("признак", f) }

// RegisterCheck добавляет проверку качества данных.
func RegisterCheck(c Check) { checks.register("проверка", c) }

// RegisterAction добавляет действие с ценой.
func RegisterAction(a Action) { actions.register("действие", a) }

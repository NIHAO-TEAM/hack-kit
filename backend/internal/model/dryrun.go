package model

import (
	"context"
	"encoding/json"
)

// DryRun — последствия правки без сохранения: сколько клиентов сменит уровень («medium->high»)
// и на сколько изменится ожидаемая потеря, в евроцентах.
type DryRun struct {
	LevelChanges           map[string]int
	ExpectedLossDeltaCents int64
	// Customers — до MaxDryRunCustomers клиентов со сменой уровня, по убыванию |ExpectedLossDeltaCents|.
	Customers      []DryRunCustomer
	CustomersTotal int
}

// MaxDryRunCustomers — сколько клиентов отдаёт предпросмотр; полное число — в CustomersTotal.
const MaxDryRunCustomers = 50

// DryRunCustomer — клиент, у которого правка меняет уровень.
type DryRunCustomer struct {
	ID                     int64
	Name                   string
	LevelFrom, LevelTo     string
	ScoreFrom, ScoreTo     float64
	ExpectedLossDeltaCents int64
}

// Ports — то, что знает ядро модели и сборка сервера, а модуль model по границам не видит.
type Ports struct {
	// Defaults — тело настроек по умолчанию.
	Defaults func() json.RawMessage
	// Known — id признаков модели.
	Known func() []string
	// Check — нарушения в весах, порогах, горизонте и priceBasis.
	Check func(body json.RawMessage) []FieldProblem
	// Preview считает последствия перехода от cur к next на текущем снимке.
	Preview func(ctx context.Context, cur, next json.RawMessage) (DryRun, error)
	// SnapshotVersion — номер версии последнего снимка (дёшево); ErrNoData, пока снимков нет.
	SnapshotVersion func(ctx context.Context) (int64, error)
	// Sensitivity считает чувствительность очереди к весам на текущем снимке по телу настроек;
	// ничего не пишет. Нет снимка — ErrNoData.
	Sensitivity func(ctx context.Context, settings json.RawMessage) (Sensitivity, error)
	// Actor — кто делает запрос (логин из сессии).
	Actor func(ctx context.Context) string
}

// Problems собирает все нарушения настроек: свои и ядра.
func (p Ports) Problems(s Settings) []FieldProblem {
	ps := s.Problems(p.Known())
	body, err := s.Marshal()
	if err != nil {
		return append(ps, FieldProblem{"", err.Error()})
	}
	return append(ps, p.Check(body)...)
}

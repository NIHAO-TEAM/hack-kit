package app

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/analytics"
	"example.com/app/internal/model"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/risk"
	"example.com/app/internal/source"
)

// Recalc — воркер пересчёта и то, что он читает. Модули друг о друге не знают, поэтому
// склейку «снимок и настройки → ядро модели» делает сборка сервера.
type Recalc struct {
	Engine   *analytics.Engine[risk.Result]
	Settings *model.Store
}

// NewRecalc собирает воркер: настройки из model_settings, снимок из таблиц организаторов.
func NewRecalc(pool *pgxpool.Pool, bus *events.Bus, clk clock.Clock) *Recalc {
	store := model.NewStore(pool, bus, ValidateSettings)
	reader := source.NewReader(pool)
	compute := func(ctx context.Context) (analytics.Computed[risk.Result], error) {
		st, settingsVersion, err := currentSettings(ctx, store)
		if err != nil {
			return analytics.Computed[risk.Result]{}, err
		}
		snap, dataVersion, err := reader.Load(ctx)
		if err != nil {
			return analytics.Computed[risk.Result]{}, err
		}
		rs := ToRiskSnapshot(snap)
		return analytics.Computed[risk.Result]{
			Result:          risk.Score(rs, st, rs.AsOf),
			DataVersion:     dataVersion,
			SettingsVersion: settingsVersion,
		}, nil
	}
	return &Recalc{
		Engine:   analytics.NewEngine(compute, bus, clk, source.TopicChanged, model.TopicChanged),
		Settings: store,
	}
}

// Start запускает воркер: первый пересчёт идёт сразу, дальше по событиям.
func (r *Recalc) Start(ctx context.Context) <-chan struct{} { return r.Engine.Start(ctx) }

// ValidateSettings проверяет тело настроек модели.
func ValidateSettings(body []byte) error {
	var st risk.Settings
	if err := json.Unmarshal(body, &st); err != nil {
		return fmt.Errorf("разбор настроек: %w", err)
	}
	return st.Validate()
}

// currentSettings читает сохранённые настройки; пока их не сохраняли — умолчания, версия 0.
func currentSettings(ctx context.Context, store *model.Store) (risk.Settings, int64, error) {
	body, version, err := store.Current(ctx)
	if err != nil {
		return risk.Settings{}, 0, err
	}
	if body == nil {
		return risk.Defaults(), version, nil
	}
	var st risk.Settings
	if err := json.Unmarshal(body, &st); err != nil {
		return risk.Settings{}, 0, fmt.Errorf("настройки версии %d: %w", version, err)
	}
	return st, version, nil
}

// ToRiskSnapshot переводит снимок источника во входные данные ядра модели.
func ToRiskSnapshot(s source.Snapshot) risk.Snapshot {
	out := risk.Snapshot{
		Accounts:     make([]risk.Account, len(s.Accounts)),
		Services:     make([]risk.Service, len(s.Services)),
		Transactions: make([]risk.Transaction, len(s.Transactions)),
	}
	if s.AsOf != nil {
		out.AsOf = *s.AsOf
	}
	for i, a := range s.Accounts {
		out.Accounts[i] = risk.Account(a)
	}
	for i, v := range s.Services {
		out.Services[i] = risk.Service(v)
	}
	for i, t := range s.Transactions {
		out.Transactions[i] = risk.Transaction(t)
	}
	return out
}

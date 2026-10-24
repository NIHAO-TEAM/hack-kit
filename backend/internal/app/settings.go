package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/auth"
	"example.com/app/internal/model"
	"example.com/app/internal/risk"
	"example.com/app/internal/source"
)

// modelPorts склеивает модуль model с ядром модели и снимком: сам model их не импортирует.
// pool может быть nil (тесты без базы): предпросмотр тогда недоступен, но model в этом
// случае отвечает 501 раньше.
func modelPorts(pool *pgxpool.Pool) model.Ports {
	var reader *source.Reader
	if pool != nil {
		reader = source.NewReader(pool)
	}
	return model.Ports{
		Defaults: func() json.RawMessage {
			b, _ := json.Marshal(risk.Defaults())
			return b
		},
		Known: knownFeatures,
		Check: func(body json.RawMessage) []model.FieldProblem {
			var st risk.Settings
			if err := json.Unmarshal(body, &st); err != nil {
				return []model.FieldProblem{{Message: "разбор настроек: " + err.Error()}}
			}
			var out []model.FieldProblem
			for _, p := range st.Problems() {
				out = append(out, model.FieldProblem{Field: p.Field, Message: p.Message})
			}
			return out
		},
		Preview: func(ctx context.Context, cur, next json.RawMessage) (model.DryRun, error) {
			return preview(ctx, reader, cur, next)
		},
		Actor: auth.LoginFrom,
	}
}

func knownFeatures() []string {
	ids := make([]string, 0)
	for id := range risk.Defaults().Weights {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// preview считает модель на текущем снимке с действующими и черновыми настройками и сравнивает.
func preview(ctx context.Context, reader *source.Reader, cur, next json.RawMessage) (model.DryRun, error) {
	var a, b risk.Settings
	if err := json.Unmarshal(cur, &a); err != nil {
		return model.DryRun{}, fmt.Errorf("действующие настройки: %w", err)
	}
	if err := json.Unmarshal(next, &b); err != nil {
		return model.DryRun{}, fmt.Errorf("черновые настройки: %w", err)
	}
	snap, _, err := reader.Load(ctx)
	if err != nil {
		return model.DryRun{}, err
	}
	rs := ToRiskSnapshot(snap)
	names := make(map[int64]string, len(rs.Accounts))
	for _, acc := range rs.Accounts {
		names[acc.ID] = acc.Email
	}
	return Diff(risk.Score(rs, a, rs.AsOf), risk.Score(rs, b, rs.AsOf), names), nil
}

// Diff сравнивает два расчёта по одному снимку: смены уровня клиентов и разницу ожидаемой потери.
// names — подписи клиентов по id (email аккаунта).
func Diff(before, after risk.Result, names map[int64]string) model.DryRun {
	prev := map[int64]risk.Customer{}
	for _, c := range before.Customers {
		if c.Level != nil {
			prev[c.ID] = c
		}
	}
	d := model.DryRun{
		LevelChanges:           map[string]int{},
		ExpectedLossDeltaCents: after.Summary.ExpectedLossCents - before.Summary.ExpectedLossCents,
	}
	for _, c := range after.Customers {
		if c.Level == nil {
			continue
		}
		was, ok := prev[c.ID]
		if !ok || *was.Level == *c.Level {
			continue
		}
		d.LevelChanges[*was.Level+"->"+*c.Level]++
		d.Customers = append(d.Customers, model.DryRunCustomer{
			ID: c.ID, Name: names[c.ID], LevelFrom: *was.Level, LevelTo: *c.Level,
			ScoreFrom: deref(was.Score), ScoreTo: deref(c.Score),
			ExpectedLossDeltaCents: c.ExpectedLossCents - was.ExpectedLossCents,
		})
	}
	d.CustomersTotal = len(d.Customers)
	sort.SliceStable(d.Customers, func(i, j int) bool {
		a, b := abs64(d.Customers[i].ExpectedLossDeltaCents), abs64(d.Customers[j].ExpectedLossDeltaCents)
		if a != b {
			return a > b
		}
		return d.Customers[i].ID < d.Customers[j].ID
	})
	if len(d.Customers) > model.MaxDryRunCustomers {
		d.Customers = d.Customers[:model.MaxDryRunCustomers]
	}
	return d
}

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

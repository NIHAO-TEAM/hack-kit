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
	return Diff(risk.Score(rs, a, rs.AsOf), risk.Score(rs, b, rs.AsOf)), nil
}

// Diff сравнивает два расчёта по одному снимку: смены уровня клиентов и разницу ожидаемой потери.
func Diff(before, after risk.Result) model.DryRun {
	levels := map[int64]string{}
	for _, c := range before.Customers {
		if c.Level != nil {
			levels[c.ID] = *c.Level
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
		if was, ok := levels[c.ID]; ok && was != *c.Level {
			d.LevelChanges[was+"->"+*c.Level]++
		}
	}
	return d
}

// Package snapcalc считает снимок тем же кодом, что пересчёт (risk.Score), и печатает по строке JSON
// на клиента — для сверки с эталоном tools/oracle/oracle.py (tools/oracle/parity.sh).
package snapcalc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"example.com/app/internal/risk"
)

// toBilling оракул не считает (см. TestActionsGolden), в сверку не входит.
const toBilling = "to_billing"

// Line — строка сверки по клиенту. Поля и имена совпадают с режимом `oracle.py --per-customer`.
type Line struct {
	ID                int64    `json:"id"`
	State             string   `json:"state"`
	Level             *string  `json:"level"`
	Score             *float64 `json:"score"`
	ExpectedLossCents int64    `json:"expectedLossCents"`
	ExposureCents     int64    `json:"exposureCents"`
	LostCents         int64    `json:"lostCents"`
	// Actions — эффект каждого действия (тип → изменение потери в центах); порядок списка не сверяется:
	// Go сортирует по выгоде, эталон отдаёт в фиксированном порядке.
	Actions map[string]int64 `json:"actions"`
	// Best — действие с наименьшим эффектом, при равенстве меньший тип по алфавиту; null, если действий нет.
	Best *string `json:"best"`
}

// Lines считает снимок на момент asOf с настройками по умолчанию; клиенты — по возрастанию id.
func Lines(snap risk.Snapshot, asOf time.Time) []Line {
	res := risk.Score(snap, risk.Defaults(), asOf)
	out := make([]Line, len(res.Customers))
	for i, c := range res.Customers {
		l := Line{ID: c.ID, State: c.State, Level: c.Level, Score: c.Score,
			ExpectedLossCents: c.ExpectedLossCents, ExposureCents: c.ExposureCents, LostCents: c.LostCents}
		l.Actions = map[string]int64{}
		for _, a := range c.Actions {
			if a.ID == toBilling {
				continue
			}
			l.Actions[a.ID] = a.LossDeltaCents
			if l.Best == nil || a.LossDeltaCents < l.Actions[*l.Best] || a.LossDeltaCents == l.Actions[*l.Best] && a.ID < *l.Best {
				id := a.ID
				l.Best = &id
			}
		}
		out[i] = l
	}
	return out
}

// Run читает снимок из r и пишет строки в w. asOf пустой — берётся из снимка.
func Run(r io.Reader, w io.Writer, asOf string) error {
	var snap risk.Snapshot
	if err := json.NewDecoder(r).Decode(&snap); err != nil {
		return fmt.Errorf("разбор снимка: %w", err)
	}
	t := snap.AsOf
	if asOf != "" {
		var err error
		if t, err = time.Parse(time.RFC3339, asOf); err != nil {
			return fmt.Errorf("as-of: %w", err)
		}
	}
	snap.AsOf = t
	bw := bufio.NewWriter(w)
	enc := json.NewEncoder(bw)
	for _, l := range Lines(snap, t) {
		if err := enc.Encode(l); err != nil {
			return err
		}
	}
	return bw.Flush()
}

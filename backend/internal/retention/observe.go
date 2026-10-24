package retention

import (
	"fmt"
	"strconv"
	"strings"

	"example.com/app/internal/analytics"
	"example.com/app/internal/platform/api/gen"
)

// Observation — наблюдение за клиентом после взятия в работу. Key одинаков у одного и того же
// наблюдения в разных пересчётах: по нему запись в историю не дублируется.
type Observation struct{ Key, Text string }

// observe сравнивает исходное состояние задачи с текущими данными клиента. seenBalance — последний
// баланс, который уже попал в историю (в начале — баланс из baseline), чтобы одно пополнение
// не записывалось дважды. Функция чистая: ни базы, ни часов.
func observe(b gen.TaskBaseline, seenBalance int64, c analytics.Customer) []Observation {
	var out []Observation
	cur := make(map[int64]gen.Service, len(c.Services))
	for _, s := range c.Services {
		cur[s.Id] = s
	}
	for _, was := range b.Services {
		id := strconv.FormatInt(was.Id, 10)
		now, ok := cur[was.Id]
		if !ok {
			out = append(out, Observation{"status:" + id + ":deleted", "Услуга " + id + " удалена"})
			continue
		}
		if !was.AutoProlong && now.AutoProlong {
			out = append(out, Observation{"autoprolong:" + id, "Включено автопродление услуги " + now.Name})
		}
		if now.ExpiresAt.After(was.ExpiresAt) || (was.Status != gen.Active && now.Status == gen.Active) {
			out = append(out, Observation{
				"renewed:" + id + ":" + strconv.FormatInt(now.ExpiresAt.Unix(), 10),
				"Услуга " + now.Name + " продлена до " + now.ExpiresAt.In(msk).Format("02.01.2006"),
			})
		}
		if now.Status != was.Status {
			switch now.Status {
			case gen.Deleted:
				out = append(out, Observation{"status:" + id + ":deleted", "Услуга " + now.Name + " удалена"})
			case gen.Blocked:
				out = append(out, Observation{"status:" + id + ":blocked", "Услуга " + now.Name + " заблокирована"})
			}
		}
	}
	if bal := c.Row.Balance; bal > seenBalance {
		out = append(out, Observation{"balance:" + strconv.FormatInt(bal, 10), "Баланс пополнен на " + euros(bal-seenBalance)})
	}
	// Скор пишем только при смене уровня риска: иначе история шумит на каждой правке коэффициента.
	if c.Row.Level != b.Level {
		loss := b.ExpectedLossCents - c.Row.ExpectedLossCents
		sign := "−"
		if loss < 0 {
			sign, loss = "+", -loss
		}
		from, to := scoreText(b.Score), scoreText(c.Row.Score)
		out = append(out, Observation{
			"level:" + string(c.Row.Level) + ":" + to,
			fmt.Sprintf("Скор %s → %s, ожидаемая потеря %s%s", from, to, sign, euros(loss)),
		})
	}
	return out
}

func scoreText(s *float64) string {
	if s == nil {
		return "—"
	}
	return strings.Replace(fmt.Sprintf("%.2f", *s), ".", ",", 1)
}

// euros — центы в тексте: «1 444,90 €».
func euros(cents int64) string {
	whole := strconv.FormatInt(cents/100, 10)
	var b []byte
	for i := range whole {
		if i > 0 && (len(whole)-i)%3 == 0 {
			b = append(b, ' ')
		}
		b = append(b, whole[i])
	}
	return fmt.Sprintf("%s,%02d €", b, cents%100)
}

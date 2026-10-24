package risk

import "fmt"

// titledCheck — проверка, которая сама подписывает результат: Subject продолжает фразу
// «N услуг …» («в ожидании оплаты дольше 5 дней»). Без него в тексте остаётся идентификатор.
type titledCheck interface {
	Check
	Subject() string
}

// checkMessage — человеческий текст результата: «2 услуги …» или «Нет услуг …».
func checkMessage(c Check, count int) string {
	subject := c.ID()
	if t, ok := c.(titledCheck); ok {
		subject = t.Subject()
	}
	if count == 0 {
		return "Нет услуг " + subject
	}
	return fmt.Sprintf("%d %s %s", count, servicesWord(count), subject)
}

func servicesWord(n int) string { return plural(n, "услуга", "услуги", "услуг") }

package analytics

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"log"
	"strconv"

	"example.com/app/internal/audit"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/httpx"
	"example.com/app/internal/platform/plural"
)

// WithAudit подключает журнал действий: выгрузка очереди записывается в него.
func (a *AnalyticsAPI) WithAudit(r audit.Recorder) *AnalyticsAPI {
	a.audit = r
	return a
}

var exportHeader = []string{
	"ID", "Email", "Баланс, €", "Пополнено всего, €", "Скор, %", "Уровень",
	"Ожидаемая потеря, €", "Главная причина", "Главное действие", "Эффект действия, €",
	"К биллингу", "Версия пересчёта", "Время модели",
}

var levelLabels = map[gen.RiskLevel]string{
	gen.High: "Высокий риск", gen.Medium: "Средний риск", gen.Low: "Низкий риск",
	gen.Unscored: "Без оценки", gen.Lost: "Потерян",
}

// ExportCustomers — GET /api/v1/customers/export.csv: та же выборка, что у списка, все строки.
func (a *AnalyticsAPI) ExportCustomers(w responseWriter, r *request, p gen.ExportCustomersParams) {
	pf := a.portfolio()
	if pf == nil {
		httpx.WriteProblem(w, r, 503, "расчёт ещё не готов")
		return
	}
	f := customerFilter{Level: p.Level, Q: p.Q, Reason: p.Reason}
	if p.Sort != nil {
		s := gen.ListCustomersParamsSort(*p.Sort)
		f.Sort = &s
	}
	if p.Order != nil && *p.Order != "asc" && *p.Order != "desc" {
		httpx.WriteProblem(w, r, 400, "order: asc или desc")
		return
	}
	if p.Order != nil {
		o := gen.ListCustomersParamsOrder(*p.Order)
		f.Order = &o
	}
	if p.Billing != nil {
		b := gen.ListCustomersParamsBilling(*p.Billing)
		f.Billing = &b
	}
	rows, bad := selectRows(pf, f)
	if bad != "" {
		httpx.WriteProblem(w, r, 400, bad)
		return
	}

	asOf := "v" + strconv.FormatInt(pf.Version, 10)
	modelTime := ""
	// Сводка берётся только от того же пересчёта, что и портфель: имя файла и колонки не расходятся.
	if o := a.latest(); o != nil && o.Version == pf.Version && !o.AsOf.IsZero() {
		asOf = o.AsOf.UTC().Format("2006-01-02T15-04")
		modelTime = o.AsOf.UTC().Format("02.01.2006 15:04")
	}

	h := w.Header()
	h.Set("Content-Type", "text/csv; charset=utf-8")
	h.Set("Content-Disposition", `attachment; filename="churn-queue-`+asOf+`.csv"`)
	h.Set("Cache-Control", "no-store")
	bw := bufio.NewWriter(w)
	_, _ = bw.WriteString("\uFEFF") // BOM: Excel читает UTF-8 без мастера импорта
	cw := csv.NewWriter(bw)
	cw.Comma = ';'
	version := strconv.FormatInt(pf.Version, 10)
	err := cw.Write(exportHeader)
	for i := 0; err == nil && i < len(rows); i++ {
		err = cw.Write(exportRecord(rows[i], version, modelTime))
	}
	cw.Flush()
	if err == nil {
		err = cw.Error()
	}
	if err == nil {
		err = bw.Flush()
	}
	if err != nil {
		log.Printf("analytics: выгрузка: %v", err)
	}
	a.recordExport(r, f, len(rows))
}

func exportRecord(c gen.CustomerRow, version, modelTime string) []string {
	score, reason, action, effect := "", "", "", ""
	if c.TopReason != nil {
		reason = *c.TopReason
	}
	if c.Score != nil {
		score = strconv.FormatFloat(*c.Score, 'f', 1, 64)
		score = replacePoint(score)
	}
	if c.TopAction != nil {
		action, effect = c.TopAction.Title, euros(c.TopAction.EffectCents)
	}
	billing := "нет"
	if isBilling(c) {
		billing = "да"
	}
	return []string{
		strconv.FormatInt(c.Id, 10), safeCell(c.Email), euros(c.Balance), euros(c.TotalReplenished),
		score, levelLabels[c.Level], euros(c.ExpectedLossCents), safeCell(reason), safeCell(action),
		effect, billing, version, modelTime,
	}
}

// euros переводит евроцентов в «1234,56» без разделителя тысяч.
func euros(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s%d,%02d", sign, cents/100, cents%100)
}

func replacePoint(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] == '.' {
			b[i] = ','
		}
	}
	return string(b)
}

// safeCell не даёт Excel исполнить текст ячейки как формулу (email и подписи приходят извне).
func safeCell(s string) string {
	if s != "" && (s[0] == '=' || s[0] == '+' || s[0] == '-' || s[0] == '@' || s[0] == '\t' || s[0] == '\r') {
		return "'" + s
	}
	return s
}

// recordExport пишет выгрузку в журнал: строки и фильтры, без email и текста поиска.
func (a *AnalyticsAPI) recordExport(r *request, f customerFilter, n int) {
	if a.audit == nil {
		return
	}
	var changes []audit.Change
	if f.Level != nil {
		changes = append(changes, audit.Change{Field: "level", From: "", To: string(*f.Level)})
	}
	if f.Billing != nil {
		changes = append(changes, audit.Change{Field: "billing", From: "", To: string(*f.Billing)})
	}
	if f.Reason != nil {
		changes = append(changes, audit.Change{Field: "reason", From: "", To: *f.Reason})
	}
	if f.Q != nil && *f.Q != "" {
		changes = append(changes, audit.Change{Field: "q", From: "", To: "задан"})
	}
	a.audit.Record(r.Context(), audit.Entry{
		Action:  audit.ActionCustomersExported,
		Summary: fmt.Sprintf("Выгрузка очереди в CSV: %d %s", n, plural.Form(n, "строка", "строки", "строк")),
		Changes: changes,
	})
}

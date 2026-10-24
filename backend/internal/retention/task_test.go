package retention

import (
	"strings"
	"testing"
	"time"

	"example.com/app/internal/platform/api/gen"
)

func TestDefaultDue(t *testing.T) {
	for _, c := range []struct{ now, want string }{
		{"2026-10-24T12:00:00Z", "2026-10-25T15:00:00Z"}, // 15:00 МСК → завтра 18:00 МСК
		{"2026-10-24T22:30:00Z", "2026-10-26T15:00:00Z"}, // уже 25.10 01:30 МСК → 26.10 18:00 МСК
		{"2026-10-24T20:59:00Z", "2026-10-25T15:00:00Z"}, // 23:59 МСК 24.10
	} {
		now, _ := time.Parse(time.RFC3339, c.now)
		if got := defaultDue(now).Format(time.RFC3339); got != c.want {
			t.Errorf("defaultDue(%s) = %s, ждали %s", c.now, got, c.want)
		}
	}
}

func str(s string) *string { return &s }

func TestApplyPatch(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	later := now.Add(24 * time.Hour)
	cur := gen.RetentionTask{Id: 1, Status: gen.InWork, Assignee: "marketer", DueAt: now.Add(time.Hour), Note: "", Version: 3}
	status := func(s gen.TaskStatus) *gen.TaskStatus { return &s }
	outcome := func(o gen.TaskOutcome) *gen.TaskOutcome { return &o }

	t.Run("ошибки", func(t *testing.T) {
		for name, c := range map[string]struct {
			p     gen.TaskPatch
			field string
		}{
			"closed без итога":     {gen.TaskPatch{Status: status(gen.Closed)}, "outcome"},
			"postponed без срока":  {gen.TaskPatch{Status: status(gen.Postponed)}, "dueAt"},
			"postponed в прошлое":  {gen.TaskPatch{Status: status(gen.Postponed), DueAt: &now}, "dueAt"},
			"итог без закрытия":    {gen.TaskPatch{Outcome: outcome(gen.Renewed)}, "outcome"},
			"неизвестный итог":     {gen.TaskPatch{Status: status(gen.Closed), Outcome: outcome("won")}, "outcome"},
			"неизвестный статус":   {gen.TaskPatch{Status: status("lost")}, "status"},
			"пустой ответственный": {gen.TaskPatch{Assignee: str("  ")}, "assignee"},
			"длинная заметка":      {gen.TaskPatch{Note: str(strings.Repeat("я", 2001))}, "note"},
		} {
			got, changes, errs := applyPatch(cur, c.p, now)
			if len(errs) == 0 || errs[0].Field != c.field {
				t.Errorf("%s: errs=%v, ждали поле %s", name, errs, c.field)
			}
			if len(changes) != 0 || got.Status != cur.Status {
				t.Errorf("%s: правка применилась: %+v %v", name, got, changes)
			}
		}
	})

	t.Run("закрытие", func(t *testing.T) {
		got, changes, errs := applyPatch(cur, gen.TaskPatch{Status: status(gen.Closed), Outcome: outcome(gen.Refused)}, now)
		if len(errs) != 0 || got.Status != gen.Closed || *got.Outcome != gen.Refused || got.ClosedAt == nil || !got.ClosedAt.Equal(now) || len(changes) != 2 {
			t.Errorf("закрытие: %+v %v %v", got, changes, errs)
		}
	})

	t.Run("отложить", func(t *testing.T) {
		got, changes, errs := applyPatch(cur, gen.TaskPatch{Status: status(gen.Postponed), DueAt: &later}, now)
		if len(errs) != 0 || got.Status != gen.Postponed || !got.DueAt.Equal(later) || len(changes) != 2 {
			t.Errorf("отложить: %+v %v %v", got, changes, errs)
		}
	})

	t.Run("без изменений", func(t *testing.T) {
		_, changes, errs := applyPatch(cur, gen.TaskPatch{Status: status(gen.InWork), Assignee: str("marketer"), DueAt: &cur.DueAt, Note: str("")}, now)
		if len(errs) != 0 || len(changes) != 0 {
			t.Errorf("правка без изменений: %v %v", changes, errs)
		}
	})

	t.Run("текст заметки не попадает в изменения", func(t *testing.T) {
		got, changes, _ := applyPatch(cur, gen.TaskPatch{Note: str("секрет")}, now)
		if got.Note != "секрет" || len(changes) != 1 || strings.Contains(changes[0].To, "секрет") || strings.Contains(changes[0].From, "секрет") {
			t.Errorf("заметка: %+v %v", got, changes)
		}
	})
}

func TestValidateEvent(t *testing.T) {
	phone := gen.Phone
	for name, c := range map[string]struct {
		e  gen.TaskEventCreate
		ok bool
	}{
		"контакт с каналом":  {gen.TaskEventCreate{Kind: gen.TaskEventCreateKindContact, Channel: &phone}, true},
		"контакт без текста": {gen.TaskEventCreate{Kind: gen.TaskEventCreateKindContact, Channel: &phone, Text: str("")}, true},
		"контакт без канала": {gen.TaskEventCreate{Kind: gen.TaskEventCreateKindContact}, false},
		"заметка":            {gen.TaskEventCreate{Kind: gen.TaskEventCreateKindNote, Text: str("позвонить")}, true},
		"пустая заметка":     {gen.TaskEventCreate{Kind: gen.TaskEventCreateKindNote}, false},
		"заметка с каналом":  {gen.TaskEventCreate{Kind: gen.TaskEventCreateKindNote, Text: str("x"), Channel: &phone}, false},
		"длинный текст":      {gen.TaskEventCreate{Kind: gen.TaskEventCreateKindNote, Text: str(strings.Repeat("я", 2001))}, false},
		"неизвестный вид":    {gen.TaskEventCreate{Kind: "call"}, false},
	} {
		if errs := validateEvent(c.e); (len(errs) == 0) != c.ok {
			t.Errorf("%s: errs=%v", name, errs)
		}
	}
}

func TestOverdue(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		status gen.TaskStatus
		due    time.Time
		want   bool
	}{
		{gen.InWork, now.Add(-time.Minute), true},
		{gen.InWork, now.Add(time.Minute), false},
		{gen.Postponed, now.Add(-time.Minute), true},
		{gen.Closed, now.Add(-time.Hour), false},
	} {
		if got := overdue(c.status, c.due, now); got != c.want {
			t.Errorf("overdue(%s, %v) = %v", c.status, c.due, got)
		}
	}
}

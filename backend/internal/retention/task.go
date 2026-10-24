// Package retention — задачи удержания: клиента берут в работу, назначают ответственного и срок,
// записывают контакты. Задачи живут отдельно от снимка: его замена их не стирает.
package retention

import (
	"strings"
	"time"
	"unicode/utf8"

	"example.com/app/internal/audit"
	"example.com/app/internal/platform/api/gen"
)

// maxTextRunes — предел длины заметки и текста контакта.
const maxTextRunes = 2000

// msk — московское время без базы часовых поясов: переходов на летнее время нет, смещение всегда +3.
var msk = time.FixedZone("MSK", 3*60*60)

// FieldError — ошибка одного поля запроса.
type FieldError struct{ Field, Message string }

// defaultDue — срок по умолчанию: завтра 18:00 МСК.
func defaultDue(now time.Time) time.Time {
	t := now.In(msk)
	return time.Date(t.Year(), t.Month(), t.Day()+1, 18, 0, 0, 0, msk).UTC()
}

func isOpen(status gen.TaskStatus) bool { return status != gen.Closed }

func overdue(status gen.TaskStatus, due, now time.Time) bool {
	return isOpen(status) && due.Before(now)
}

func validStatus(s gen.TaskStatus) bool {
	switch s {
	case gen.InWork, gen.Contacted, gen.Postponed, gen.Closed:
		return true
	}
	return false
}

func validOutcome(o gen.TaskOutcome) bool {
	switch o {
	case gen.Renewed, gen.Refused, gen.NoAnswer, gen.NotNeeded:
		return true
	}
	return false
}

func validChannel(c gen.TaskChannel) bool {
	switch c {
	case gen.Mail, gen.Phone, gen.Messenger, gen.Other:
		return true
	}
	return false
}

func tooLong(s string) bool { return utf8.RuneCountInString(s) > maxTextRunes }

// applyPatch применяет правку к копии задачи и возвращает, что поменялось. Пустой список изменений —
// правка ничего не меняет. Версию и автора правки проставляет вызывающий.
func applyPatch(cur gen.RetentionTask, p gen.TaskPatch, now time.Time) (gen.RetentionTask, []audit.Change, []FieldError) {
	var errs []FieldError
	next := cur
	var changes []audit.Change
	change := func(field, from, to string) {
		changes = append(changes, audit.Change{Field: field, From: from, To: to})
	}

	status := cur.Status
	if p.Status != nil {
		if !validStatus(*p.Status) {
			errs = append(errs, FieldError{"status", "неизвестный статус"})
		} else {
			status = *p.Status
		}
	}
	if p.Outcome != nil && !validOutcome(*p.Outcome) {
		errs = append(errs, FieldError{"outcome", "неизвестный итог"})
	}
	if status == gen.Closed {
		if p.Outcome == nil {
			errs = append(errs, FieldError{"outcome", "для закрытия нужен итог"})
		}
	} else if p.Outcome != nil {
		errs = append(errs, FieldError{"outcome", "итог указывают только при закрытии"})
	}
	// Срок нужен при переходе в «отложено»; правка уже отложенной задачи (ответственный, заметка) его не требует.
	if p.Status != nil && status == gen.Postponed {
		if p.DueAt == nil {
			errs = append(errs, FieldError{"dueAt", "для откладывания нужен новый срок"})
		} else if !p.DueAt.After(now) {
			errs = append(errs, FieldError{"dueAt", "новый срок должен быть в будущем"})
		}
	}
	if p.Assignee != nil && strings.TrimSpace(*p.Assignee) == "" {
		errs = append(errs, FieldError{"assignee", "логин не может быть пустым"})
	}
	if p.Note != nil && tooLong(*p.Note) {
		errs = append(errs, FieldError{"note", "заметка длиннее 2000 символов"})
	}
	if len(errs) > 0 {
		return cur, nil, errs
	}

	if status != cur.Status {
		change("status", string(cur.Status), string(status))
		next.Status = status
	}
	if status == gen.Closed {
		next.Outcome = p.Outcome
		change("outcome", "", string(*p.Outcome))
		closed := now
		next.ClosedAt = &closed
	}
	if p.Assignee != nil {
		if a := strings.TrimSpace(*p.Assignee); a != cur.Assignee {
			change("assignee", cur.Assignee, a)
			next.Assignee = a
		}
	}
	if p.DueAt != nil && !p.DueAt.Equal(cur.DueAt) {
		change("dueAt", cur.DueAt.UTC().Format(time.RFC3339), p.DueAt.UTC().Format(time.RFC3339))
		next.DueAt = p.DueAt.UTC()
	}
	if p.Note != nil && *p.Note != cur.Note {
		// Текст заметки в журнал и историю изменений не кладём: он остаётся в самой задаче.
		change("note", "", "изменена")
		next.Note = *p.Note
	}
	return next, changes, nil
}

// validateEvent проверяет запись в историю: контакту нужен канал.
func validateEvent(e gen.TaskEventCreate) []FieldError {
	var errs []FieldError
	text := ""
	if e.Text != nil {
		text = *e.Text
	}
	if tooLong(text) {
		errs = append(errs, FieldError{"text", "текст длиннее 2000 символов"})
	}
	switch e.Kind {
	case gen.TaskEventCreateKindContact:
		if e.Channel == nil || !validChannel(*e.Channel) {
			errs = append(errs, FieldError{"channel", "для контакта нужен канал: mail, phone, messenger или other"})
		}
	case gen.TaskEventCreateKindNote:
		if strings.TrimSpace(text) == "" {
			errs = append(errs, FieldError{"text", "заметка не может быть пустой"})
		}
		if e.Channel != nil {
			errs = append(errs, FieldError{"channel", "канал указывают только у контакта"})
		}
	default:
		errs = append(errs, FieldError{"kind", "kind: contact или note"})
	}
	return errs
}

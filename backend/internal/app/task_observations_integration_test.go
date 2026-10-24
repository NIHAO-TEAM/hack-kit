//go:build integration

package app_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"example.com/app/internal/auth"
	"example.com/app/internal/platform/api/gen"
)

// observations ждёт, пока в истории задачи станет want наблюдений, и возвращает их тексты.
func observations(t *testing.T, c *gen.ClientWithResponses, id int64, want int) []string {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		res, err := c.GetTaskWithResponse(context.Background(), id)
		if err != nil || res.JSON200 == nil || res.JSON200.Events == nil {
			t.Fatalf("задача %d: %v", id, err)
		}
		var out []string
		for _, e := range *res.JSON200.Events {
			if e.Kind == gen.TaskEventKindObserved {
				if e.Actor != "system" {
					t.Errorf("автор наблюдения %q", e.Actor)
				}
				out = append(out, e.Text)
			}
		}
		if len(out) >= want || time.Now().After(deadline) {
			return out
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func emulate(t *testing.T, c *gen.ClientWithResponses, a ...gen.EmulatorAction) {
	t.Helper()
	res, err := c.EmulatorActionsWithResponse(context.Background(), gen.EmulatorActionsRequest{Actions: a})
	if err != nil || res.JSON200 == nil {
		t.Fatalf("эмулятор: %v %d %s", err, res.StatusCode(), res.Body)
	}
}

func TestTaskObservations(t *testing.T) {
	r := taskRig(t)
	marketer, analyst := r.As(t, auth.RoleMarketer), r.As(t, auth.RoleAnalyst)

	t9, t1 := take(t, marketer, 9).JSON201, take(t, marketer, 1).JSON201
	if t9 == nil || t1 == nil {
		t.Fatal("не взяли клиентов 9 и 1")
	}

	cents := int64(60000)
	emulate(t, analyst, gen.EmulatorAction{ActionId: "top_up", CustomerId: 9, Cents: &cents})
	got := observations(t, marketer, t9.Id, 1)
	if len(got) != 1 || got[0] != "Баланс пополнен на 600,00 €" {
		t.Fatalf("наблюдения по клиенту 9: %q", got)
	}

	// Пересчёт без изменений у клиента 9 (правка чужого клиента) нового не пишет.
	emulate(t, analyst, gen.EmulatorAction{ActionId: "enable_autoprolong", CustomerId: 1})
	got1 := observations(t, marketer, t1.Id, 1)
	if len(got1) == 0 || !strings.HasPrefix(got1[0], "Включено автопродление услуги ") {
		t.Fatalf("наблюдения по клиенту 1: %q", got1)
	}
	if again := observations(t, marketer, t9.Id, 2); len(again) != 1 {
		t.Errorf("после повторного пересчёта у клиента 9: %q", again)
	}
	// Второе пополнение — отдельное наблюдение только на добавленную сумму.
	more := int64(1500)
	emulate(t, analyst, gen.EmulatorAction{ActionId: "top_up", CustomerId: 9, Cents: &more})
	if two := observations(t, marketer, t9.Id, 2); len(two) != 2 || two[1] != "Баланс пополнен на 15,00 €" {
		t.Errorf("после второго пополнения: %q", two)
	}

	// В списке — последнее наблюдение.
	one, _ := marketer.GetTaskWithResponse(context.Background(), t9.Id)
	if one.JSON200 == nil || one.JSON200.LastObservation == nil || *one.JSON200.LastObservation != "Баланс пополнен на 15,00 €" {
		t.Errorf("lastObservation: %s", one.Body)
	}

	// Закрытой задаче наблюдения не дописываются.
	closed := patchTask(t, marketer, t1.Id, gen.TaskPatch{Version: t1.Version, Status: statusPtr(gen.Closed), Outcome: outcomePtr(gen.Renewed)})
	if closed.JSON200 == nil {
		t.Fatalf("закрыть: %d %s", closed.StatusCode(), closed.Body)
	}
	before := len(observations(t, marketer, t1.Id, 0))
	emulate(t, analyst, gen.EmulatorAction{ActionId: "top_up", CustomerId: 1, Cents: &cents})
	emulate(t, analyst, gen.EmulatorAction{ActionId: "top_up", CustomerId: 9, Cents: &cents}) // метка: к её наблюдению пересчёт уже прошёл
	observations(t, marketer, t9.Id, 3)
	if after := observations(t, marketer, t1.Id, 0); len(after) != before {
		t.Errorf("в закрытую задачу дописано: %q", after)
	}
}

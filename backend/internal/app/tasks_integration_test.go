//go:build integration

package app_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"example.com/app/internal/analytics"
	"example.com/app/internal/auth"
	"example.com/app/internal/platform/api/gen"
)

// taskRig — стенд с посчитанным портфелем организаторов.
func taskRig(t *testing.T) *rig {
	t.Helper()
	r := newRig(t, true)
	r.StartRecalc(t)
	c := r.As(t, auth.RoleMarketer)
	deadline := time.Now().Add(10 * time.Second)
	for {
		res, err := c.GetCustomerWithResponse(context.Background(), 9)
		if err == nil && res.JSON200 != nil {
			return r
		}
		if time.Now().After(deadline) {
			t.Fatal("портфель не посчитан")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func take(t *testing.T, c *gen.ClientWithResponses, id int64) *gen.TakeCustomerTaskResponse {
	t.Helper()
	res, err := c.TakeCustomerTaskWithResponse(context.Background(), id, gen.TakeCustomerTaskJSONRequestBody{})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestTaskTakeAndBaseline(t *testing.T) {
	r := taskRig(t)
	ctx := context.Background()
	marketer := r.As(t, auth.RoleMarketer)

	res := take(t, marketer, 9)
	if res.JSON201 == nil {
		t.Fatalf("взять №9: %d %s", res.StatusCode(), res.Body)
	}
	task := res.JSON201
	// Часы стенда: 24.10 12:00 UTC (15:00 МСК) → срок 25.10 18:00 МСК = 15:00 UTC.
	if want := time.Date(2026, 10, 25, 15, 0, 0, 0, time.UTC); !task.DueAt.Equal(want) {
		t.Errorf("срок %v, ждали %v", task.DueAt, want)
	}
	if task.Status != gen.InWork || task.Assignee != auth.RoleMarketer || task.CustomerId != 9 || task.Overdue || task.CustomerMissing {
		t.Errorf("задача: %+v", task)
	}
	if task.Events == nil || len(*task.Events) != 1 || (*task.Events)[0].Kind != gen.TaskEventKindCreated {
		t.Errorf("история: %+v", task.Events)
	}

	// Исходное состояние совпадает с карточкой.
	card, err := marketer.GetCustomerWithResponse(ctx, 9)
	if err != nil || card.JSON200 == nil {
		t.Fatalf("карточка: %v %d", err, card.StatusCode())
	}
	b, row := task.Baseline, card.JSON200.Customer
	if b.Version != card.JSON200.Version || b.Level != row.Level || b.ExpectedLossCents != row.ExpectedLossCents || b.Balance != row.Balance {
		t.Errorf("исходное состояние %+v, карточка %+v", b, row)
	}
	if (b.Score == nil) != (row.Score == nil) || (b.Score != nil && *b.Score != *row.Score) {
		t.Errorf("скор %v, в карточке %v", b.Score, row.Score)
	}
	if b.TopAction == nil || row.TopAction == nil || *b.TopAction != *row.TopAction {
		t.Errorf("главное действие %+v, в карточке %+v", b.TopAction, row.TopAction)
	}
	if b.AsOf == nil || len(b.Services) != len(card.JSON200.Services) || len(b.Services) == 0 {
		t.Fatalf("услуги в исходном состоянии: %+v, в карточке %d", b.Services, len(card.JSON200.Services))
	}
	for i, s := range card.JSON200.Services {
		got := b.Services[i]
		if got.Id != s.Id || got.Status != s.Status || got.AutoProlong != s.AutoProlong || !got.ExpiresAt.Equal(s.ExpiresAt) {
			t.Errorf("услуга %d: %+v, в карточке %+v", s.Id, got, s)
		}
	}

	// Краткая задача в карточке и в строке очереди.
	if row.Task == nil || row.Task.Id != task.Id || row.Task.Assignee != auth.RoleMarketer || row.Task.Status != gen.InWork {
		t.Errorf("task в карточке: %+v", row.Task)
	}
	list, _ := marketer.ListCustomersWithResponse(ctx, &gen.ListCustomersParams{})
	if list.JSON200 == nil || list.JSON200.Items[0].Id != 9 || list.JSON200.Items[0].Task == nil || list.JSON200.Items[1].Task != nil {
		t.Errorf("task в очереди: %s", list.Body)
	}

	// Повтор: 409 с той же задачей.
	again := take(t, marketer, 9)
	if again.StatusCode() != http.StatusConflict {
		t.Fatalf("повтор: %d %s", again.StatusCode(), again.Body)
	}
	var conflict gen.TaskConflict
	if err := json.Unmarshal(again.Body, &conflict); err != nil || conflict.Task == nil || conflict.Task.Id != task.Id {
		t.Errorf("тело 409: %s", again.Body)
	}
	// Неизвестный клиент.
	if res := take(t, marketer, 424242); res.StatusCode() != http.StatusNotFound {
		t.Errorf("неизвестный клиент: %d", res.StatusCode())
	}
}

func TestTaskTakeRace(t *testing.T) {
	r := taskRig(t)
	marketer, jury := r.As(t, auth.RoleMarketer), r.As(t, auth.RoleJury)
	codes := make([]int, 8)
	var wg sync.WaitGroup
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := marketer
			if i%2 == 1 {
				c = jury
			}
			codes[i] = take(t, c, 1).StatusCode()
		}()
	}
	wg.Wait()
	created := 0
	for _, c := range codes {
		switch c {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
		default:
			t.Errorf("коды: %v", codes)
		}
	}
	var n int
	if err := r.Pool.QueryRow(context.Background(), "SELECT count(*) FROM retention_tasks WHERE customer_id = 1").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if created != 1 || n != 1 {
		t.Errorf("создано %d, строк %d, коды %v", created, n, codes)
	}
}

func patchTask(t *testing.T, c *gen.ClientWithResponses, id int64, p gen.TaskPatch) *gen.PatchTaskResponse {
	t.Helper()
	res, err := c.PatchTaskWithResponse(context.Background(), id, p)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestTaskPatch(t *testing.T) {
	r := taskRig(t)
	ctx := context.Background()
	marketer, jury := r.As(t, auth.RoleMarketer), r.As(t, auth.RoleJury)
	task := take(t, marketer, 9).JSON201
	if task == nil {
		t.Fatal("задача не создана")
	}

	status := func(s gen.TaskStatus) *gen.TaskStatus { return &s }
	// Проверки: закрыть без итога, отложить без срока, отложить в прошлое, итог без закрытия.
	past := r.Clock.Now().Add(-time.Hour)
	for name, p := range map[string]gen.TaskPatch{
		"closed без outcome":  {Version: task.Version, Status: status(gen.Closed)},
		"postponed без срока": {Version: task.Version, Status: status(gen.Postponed)},
		"postponed в прошлом": {Version: task.Version, Status: status(gen.Postponed), DueAt: &past},
		"outcome без closed":  {Version: task.Version, Outcome: outcomePtr(gen.Renewed)},
		"неизвестный статус":  {Version: task.Version, Status: status("lost")},
	} {
		if res := patchTask(t, marketer, task.Id, p); res.StatusCode() != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, res.StatusCode(), res.Body)
		}
	}
	if res := patchTask(t, marketer, task.Id, gen.TaskPatch{Version: task.Version, Assignee: strPtr("nobody")}); res.StatusCode() != http.StatusBadRequest {
		t.Errorf("чужой ответственный: %d", res.StatusCode())
	}

	// Отклонённые правки данных не меняют.
	cur, _ := marketer.GetTaskWithResponse(ctx, task.Id)
	if cur.JSON200 == nil || cur.JSON200.Version != task.Version || cur.JSON200.Status != gen.InWork {
		t.Fatalf("после отказов: %s", cur.Body)
	}

	// Правка: ответственный — jury, статус «Связались».
	ok := patchTask(t, marketer, task.Id, gen.TaskPatch{Version: task.Version, Status: status(gen.Contacted), Assignee: strPtr(auth.RoleJury), Note: strPtr("позвонить")})
	if ok.JSON200 == nil || ok.JSON200.Version != task.Version+1 || ok.JSON200.Assignee != auth.RoleJury || ok.JSON200.Status != gen.Contacted || ok.JSON200.Note != "позвонить" {
		t.Fatalf("правка: %d %s", ok.StatusCode(), ok.Body)
	}
	ev := *ok.JSON200.Events
	if len(ev) != 2 || ev[1].Kind != gen.TaskEventKindChanged || len(ev[1].Changes) != 3 {
		t.Errorf("история правки: %+v", ev)
	}

	// Устаревшая версия: 409 с логином того, кто изменил, данные не меняются.
	stale := patchTask(t, jury, task.Id, gen.TaskPatch{Version: task.Version, Note: strPtr("моя заметка")})
	if stale.StatusCode() != http.StatusConflict {
		t.Fatalf("устаревшая версия: %d %s", stale.StatusCode(), stale.Body)
	}
	var conflict gen.TaskConflict
	if err := json.Unmarshal(stale.Body, &conflict); err != nil ||
		conflict.Detail == nil || !strings.Contains(*conflict.Detail, "marketer") || conflict.Task == nil || conflict.Task.Note != "позвонить" {
		t.Errorf("тело 409: %s", stale.Body)
	}

	// Закрыть с итогом; после закрытия правки и записи в историю — 409.
	closed := patchTask(t, jury, task.Id, gen.TaskPatch{Version: ok.JSON200.Version, Status: status(gen.Closed), Outcome: outcomePtr(gen.Renewed)})
	if closed.JSON200 == nil || closed.JSON200.Status != gen.Closed || closed.JSON200.Outcome == nil || *closed.JSON200.Outcome != gen.Renewed || closed.JSON200.ClosedAt == nil {
		t.Fatalf("закрытие: %d %s", closed.StatusCode(), closed.Body)
	}
	if res := patchTask(t, jury, task.Id, gen.TaskPatch{Version: closed.JSON200.Version, Note: strPtr("поздно")}); res.StatusCode() != http.StatusConflict {
		t.Errorf("правка закрытой: %d", res.StatusCode())
	}
	if res, _ := jury.AddTaskEventWithResponse(ctx, task.Id, gen.TaskEventCreate{Kind: gen.TaskEventCreateKindNote, Text: strPtr("поздно")}); res.StatusCode() != http.StatusConflict {
		t.Errorf("запись в закрытую: %d", res.StatusCode())
	}
	// Закрытая задача не мешает взять клиента заново и не показывается в строке.
	card, _ := marketer.GetCustomerWithResponse(ctx, 9)
	if card.JSON200 == nil || card.JSON200.Customer.Task != nil {
		t.Errorf("task закрытой в карточке: %s", card.Body)
	}
	if again := take(t, marketer, 9); again.JSON201 == nil || again.JSON201.Id == task.Id {
		t.Errorf("повторное взятие после закрытия: %d %s", again.StatusCode(), again.Body)
	}
}

func outcomePtr(o gen.TaskOutcome) *gen.TaskOutcome { return &o }
func strPtr(s string) *string                       { return &s }

func TestTaskEventsAndList(t *testing.T) {
	r := taskRig(t)
	ctx := context.Background()
	marketer, jury := r.As(t, auth.RoleMarketer), r.As(t, auth.RoleJury)
	t9, t1 := take(t, marketer, 9).JSON201, take(t, jury, 1).JSON201
	if t9 == nil || t1 == nil {
		t.Fatal("задачи не созданы")
	}

	// Контакт без канала — 400; контакт и заметка идут в историю по порядку.
	if res, _ := marketer.AddTaskEventWithResponse(ctx, t9.Id, gen.TaskEventCreate{Kind: gen.TaskEventCreateKindContact}); res.StatusCode() != http.StatusBadRequest {
		t.Errorf("контакт без канала: %d", res.StatusCode())
	}
	ch := gen.Phone
	if res, _ := marketer.AddTaskEventWithResponse(ctx, t9.Id, gen.TaskEventCreate{Kind: gen.TaskEventCreateKindContact, Channel: &ch}); res.StatusCode() != http.StatusCreated {
		t.Fatalf("контакт: %d %s", res.StatusCode(), res.Body)
	}
	r.Clock.Advance(time.Minute)
	if res, _ := jury.AddTaskEventWithResponse(ctx, t9.Id, gen.TaskEventCreate{Kind: gen.TaskEventCreateKindNote, Text: strPtr("просит скидку")}); res.StatusCode() != http.StatusCreated {
		t.Fatalf("заметка: %d %s", res.StatusCode(), res.Body)
	}
	got, _ := marketer.GetTaskWithResponse(ctx, t9.Id)
	if got.JSON200 == nil || got.JSON200.Events == nil {
		t.Fatalf("задача: %s", got.Body)
	}
	ev := *got.JSON200.Events
	if len(ev) != 3 || ev[0].Kind != gen.TaskEventKindCreated || ev[1].Kind != gen.TaskEventKindContact || ev[1].Channel == nil || *ev[1].Channel != gen.Phone ||
		ev[2].Kind != gen.TaskEventKindNote || ev[2].Text != "просит скидку" || ev[2].Actor != auth.RoleJury {
		t.Errorf("история: %+v", ev)
	}
	if res, _ := marketer.GetTaskWithResponse(ctx, 999999); res.StatusCode() != http.StatusNotFound {
		t.Errorf("неизвестная задача: %d", res.StatusCode())
	}

	// Список: фильтры по ответственному, статусу, просрочке; в списке истории нет.
	list := func(p gen.ListTasksParams) *gen.TaskPage {
		res, err := marketer.ListTasksWithResponse(ctx, &p)
		if err != nil || res.JSON200 == nil {
			t.Fatalf("список: %v %d %s", err, res.StatusCode(), res.Body)
		}
		return res.JSON200
	}
	if p := list(gen.ListTasksParams{}); p.Total != 2 || p.Items[0].Id != t9.Id || p.Items[0].Events != nil {
		t.Errorf("все задачи: %+v", p)
	}
	me := "me"
	if p := list(gen.ListTasksParams{Assignee: &me}); p.Total != 1 || p.Items[0].Id != t9.Id {
		t.Errorf("мои: %+v", p)
	}
	closedStatus := gen.Closed
	if p := list(gen.ListTasksParams{Status: &closedStatus}); p.Total != 0 {
		t.Errorf("закрытые: %+v", p)
	}
	yes := true
	if p := list(gen.ListTasksParams{Overdue: &yes}); p.Total != 0 {
		t.Errorf("просроченные до срока: %+v", p)
	}
	r.Clock.Advance(48 * time.Hour)
	marketer = r.As(t, auth.RoleMarketer) // сессия жила меньше двух суток
	p := list(gen.ListTasksParams{Overdue: &yes})
	if p.Total != 2 || !p.Items[0].Overdue {
		t.Errorf("просроченные после срока: %+v", p)
	}
	one := 1
	if p := list(gen.ListTasksParams{Limit: &one}); len(p.Items) != 1 || p.Total != 2 {
		t.Errorf("страница: %+v", p)
	}
	bad := 0
	if res, _ := marketer.ListTasksWithResponse(ctx, &gen.ListTasksParams{Limit: &bad}); res.StatusCode() != http.StatusBadRequest {
		t.Errorf("limit 0: %d", res.StatusCode())
	}

	// Исполнители: только те, кому можно назначать.
	as, _ := marketer.ListTaskAssigneesWithResponse(ctx)
	if as.JSON200 == nil {
		t.Fatalf("исполнители: %s", as.Body)
	}
	for _, a := range as.JSON200.Items {
		if a.Role == gen.Role(auth.RoleAnalyst) {
			t.Errorf("аналитик в исполнителях: %+v", a)
		}
	}
	if len(as.JSON200.Items) < 2 {
		t.Errorf("исполнители: %+v", as.JSON200.Items)
	}

	// Фильтр клиентов: none/open/mine, total после фильтра.
	customers := func(f gen.ListCustomersParamsTask, c *gen.ClientWithResponses) *gen.CustomerPage {
		res, err := c.ListCustomersWithResponse(ctx, &gen.ListCustomersParams{Task: &f})
		if err != nil || res.JSON200 == nil {
			t.Fatalf("клиенты task=%s: %v %d %s", f, err, res.StatusCode(), res.Body)
		}
		return res.JSON200
	}
	if pg := customers("open", marketer); pg.Total != 2 || len(pg.Items) != 2 {
		t.Errorf("open: %+v", pg.Total)
	}
	if pg := customers("none", marketer); pg.Total != 8 || len(pg.Items) != 8 {
		t.Errorf("none: %d", pg.Total)
	}
	if pg := customers("mine", marketer); pg.Total != 1 || pg.Items[0].Id != 9 {
		t.Errorf("mine: %+v", pg)
	}
	if pg := customers("mine", jury); pg.Total != 1 || pg.Items[0].Id != 1 {
		t.Errorf("mine jury: %+v", pg)
	}
	badFilter := gen.ListCustomersParamsTask("all")
	if res, _ := marketer.ListCustomersWithResponse(ctx, &gen.ListCustomersParams{Task: &badFilter}); res.StatusCode() != http.StatusBadRequest {
		t.Errorf("task=all: %d", res.StatusCode())
	}
}

func TestTaskPermissions(t *testing.T) {
	r := taskRig(t)
	ctx := context.Background()
	task := take(t, r.As(t, auth.RoleMarketer), 9).JSON201
	analyst := r.As(t, auth.RoleAnalyst)

	if res := take(t, analyst, 1); res.StatusCode() != http.StatusForbidden {
		t.Errorf("аналитик берёт в работу: %d", res.StatusCode())
	}
	if res := patchTask(t, analyst, task.Id, gen.TaskPatch{Version: task.Version, Note: strPtr("x")}); res.StatusCode() != http.StatusForbidden {
		t.Errorf("аналитик правит: %d", res.StatusCode())
	}
	if res, _ := analyst.AddTaskEventWithResponse(ctx, task.Id, gen.TaskEventCreate{Kind: gen.TaskEventCreateKindNote, Text: strPtr("x")}); res.StatusCode() != http.StatusForbidden {
		t.Errorf("аналитик пишет в историю: %d", res.StatusCode())
	}
	var n int
	_ = r.Pool.QueryRow(ctx, "SELECT count(*) FROM retention_tasks").Scan(&n)
	if n != 1 {
		t.Errorf("задач %d, ждали 1: запись аналитика прошла", n)
	}
	// Чтение открыто любой роли.
	if res, _ := analyst.GetTaskWithResponse(ctx, task.Id); res.JSON200 == nil {
		t.Errorf("аналитик читает: %d", res.StatusCode())
	}
	if res, _ := analyst.ListTasksWithResponse(ctx, &gen.ListTasksParams{}); res.JSON200 == nil {
		t.Errorf("аналитик читает список: %d", res.StatusCode())
	}
	// Без сессии — 401.
	anon := r.Anonymous(t)
	if res := take(t, anon, 1); res.StatusCode() != http.StatusUnauthorized {
		t.Errorf("take без сессии: %d", res.StatusCode())
	}
	if res, _ := anon.ListTasksWithResponse(ctx, &gen.ListTasksParams{}); res.StatusCode() != http.StatusUnauthorized {
		t.Errorf("список без сессии: %d", res.StatusCode())
	}
	if res, _ := anon.GetTaskWithResponse(ctx, task.Id); res.StatusCode() != http.StatusUnauthorized {
		t.Errorf("задача без сессии: %d", res.StatusCode())
	}
	// Администратор может.
	if res := take(t, r.As(t, auth.RoleAdmin), 1); res.JSON201 == nil {
		t.Errorf("администратор берёт в работу: %d %s", res.StatusCode(), res.Body)
	}
}

// reloadWith грузит сид организаторов, изменив его: mutate правит разобранный снимок.
func (r *rig) reloadWith(t *testing.T, mutate func(map[string][]map[string]any)) {
	t.Helper()
	raw, err := os.ReadFile("../source/seed/organizers.json")
	if err != nil {
		t.Fatal(err)
	}
	var snap map[string][]map[string]any
	if err := json.Unmarshal(raw, &snap); err != nil {
		t.Fatal(err)
	}
	mutate(snap)
	body, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	res, err := r.WithBearer(t, "source-token").PutSnapshotWithBodyWithResponse(context.Background(), "application/json", bytes.NewReader(body))
	if err != nil || res.StatusCode() != http.StatusOK {
		t.Fatalf("PUT snapshot: %v %d %s", err, res.StatusCode(), res.Body)
	}
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("не дождались: %s", what)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func TestTaskSurvivesSnapshotReplace(t *testing.T) {
	r := taskRig(t)
	ctx := context.Background()
	marketer := r.As(t, auth.RoleMarketer)
	t9, t1 := take(t, marketer, 9).JSON201, take(t, marketer, 1).JSON201
	if t9 == nil || t1 == nil {
		t.Fatal("задачи не созданы")
	}

	// Тот же снимок: задачи на месте и привязаны к клиентам.
	r.reload(t)
	if got, _ := marketer.GetTaskWithResponse(ctx, t9.Id); got.JSON200 == nil || got.JSON200.CustomerMissing {
		t.Errorf("после повторной загрузки: %s", got.Body)
	}

	// Клиент 9 пропал из снимка; у клиента 1 другой email.
	r.reloadWith(t, func(s map[string][]map[string]any) {
		var services []map[string]any
		gone := map[float64]bool{}
		for _, sv := range s["services"] {
			if sv["ownerId"] == float64(9) {
				gone[sv["id"].(float64)] = true
				continue
			}
			services = append(services, sv)
		}
		var accounts []map[string]any
		for _, a := range s["accounts"] {
			switch a["id"] {
			case float64(9):
				continue
			case float64(1):
				a["email"] = "someone-else@example.test"
			}
			accounts = append(accounts, a)
		}
		var txs []map[string]any
		for _, tx := range s["transactions"] {
			if !gone[tx["serviceId"].(float64)] {
				txs = append(txs, tx)
			}
		}
		s["accounts"], s["services"], s["transactions"] = accounts, services, txs
	})
	eventually(t, "клиент 9 пропал из портфеля", func() bool {
		res, _ := marketer.GetCustomerWithResponse(ctx, 9)
		return res.StatusCode() == http.StatusNotFound
	})
	for _, task := range []*gen.RetentionTask{t9, t1} {
		got, _ := marketer.GetTaskWithResponse(ctx, task.Id)
		if got.JSON200 == nil || !got.JSON200.CustomerMissing || got.JSON200.Status != gen.InWork || got.JSON200.Baseline.Balance != task.Baseline.Balance {
			t.Errorf("задача %d после замены снимка: %s", task.Id, got.Body)
		}
	}
	// К чужому email той же строки задача не привязывается.
	card, _ := marketer.GetCustomerWithResponse(ctx, 1)
	if card.JSON200 == nil || card.JSON200.Customer.Task != nil {
		t.Errorf("задача прилипла к клиенту с другим email: %s", card.Body)
	}
	if res, _ := marketer.ListCustomersWithResponse(ctx, &gen.ListCustomersParams{}); res.JSON200 == nil {
		t.Fatal("список клиентов")
	} else {
		for _, row := range res.JSON200.Items {
			if row.Task != nil {
				t.Errorf("задача в строке клиента %d после замены снимка", row.Id)
			}
		}
	}
}

func TestTaskAuditAndSSE(t *testing.T) {
	r := taskRig(t)
	ctx := context.Background()
	marketer := r.As(t, auth.RoleMarketer)

	bus, cancel := r.Bus.Subscribe(analytics.TopicTasks)
	defer cancel()

	// SSE-поток настоящим запросом: событие tasks приходит после записи.
	login, err := r.Client.LoginWithResponse(ctx, gen.LoginRequest{Login: auth.RoleMarketer, Password: "pw-marketer"})
	if err != nil || login.JSON200 == nil || login.JSON200.Token == nil {
		t.Fatalf("вход: %v", err)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, r.URL+"/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer "+*login.JSON200.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	waitLine := func(prefix string) string {
		t.Helper()
		timeout := time.After(10 * time.Second)
		for {
			select {
			case l, ok := <-lines:
				if !ok {
					t.Fatalf("поток закрыт до %q", prefix)
				}
				if strings.HasPrefix(l, prefix) {
					return l
				}
			case <-timeout:
				t.Fatalf("нет строки %q", prefix)
			}
		}
	}
	waitLine(": connected")

	task := take(t, marketer, 9).JSON201
	if task == nil {
		t.Fatal("задача не создана")
	}
	waitLine("event: tasks")
	data := waitLine("data: ")
	var ev analytics.TaskEvent
	if err := json.Unmarshal([]byte(strings.TrimPrefix(data, "data: ")), &ev); err != nil || ev.Type != "created" || ev.TaskID != task.Id || ev.CustomerID != 9 {
		t.Errorf("событие SSE: %q", data)
	}
	if got := <-bus; got.Topic != analytics.TopicTasks {
		t.Errorf("шина: %+v", got)
	}

	patchTask(t, marketer, task.Id, gen.TaskPatch{Version: task.Version, Status: statusPtr(gen.Contacted), Note: strPtr("секретный текст")})
	ch := gen.Mail
	if res, _ := marketer.AddTaskEventWithResponse(ctx, task.Id, gen.TaskEventCreate{Kind: gen.TaskEventCreateKindContact, Channel: &ch, Text: strPtr("секретный текст")}); res.StatusCode() != http.StatusCreated {
		t.Fatalf("контакт: %d", res.StatusCode())
	}

	// Журнал: три записи, без email клиента и текстов.
	res, err := r.As(t, auth.RoleAnalyst).ListAuditWithResponse(ctx, &gen.ListAuditParams{})
	if err != nil || res.JSON200 == nil {
		t.Fatalf("журнал: %v %d", err, res.StatusCode())
	}
	seen := map[gen.AuditEntryAction]gen.AuditEntry{}
	for _, e := range res.JSON200.Items {
		seen[e.Action] = e
	}
	for _, a := range []gen.AuditEntryAction{gen.TaskCreated, gen.TaskChanged, gen.TaskRecorded} {
		e, ok := seen[a]
		if !ok || e.Actor != auth.RoleMarketer {
			t.Errorf("нет записи %s: %+v", a, e)
		}
	}
	if raw := string(res.Body); strings.Contains(raw, "example.test") || strings.Contains(raw, "секретный текст") {
		t.Errorf("в журнал попали email или текст: %s", raw)
	}
	if ch := seen[gen.TaskChanged].Changes; len(ch) != 2 {
		t.Errorf("изменения в журнале: %+v", ch)
	}
}

func statusPtr(s gen.TaskStatus) *gen.TaskStatus { return &s }

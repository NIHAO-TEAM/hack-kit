package auth

import (
	"strings"
	"testing"

	"example.com/app/internal/platform/api/gen"
)

// Ожидаемая матрица записана отдельно от кода: ключ — operationId из контракта.
var wantAccess = map[string]struct{ marketer, analyst, jury, admin bool }{
	"getMe": {true, true, true, true}, "logout": {true, true, true, true},
	"listCustomers": {true, true, true, true}, "getCustomer": {true, true, true, true},
	"getSummary": {true, true, true, true}, "getChurnChart": {true, true, true, true},
	"getEvents": {true, true, true, true}, "getSourceStatus": {true, true, true, true},
	"getModelSettings": {true, true, true, true},
	"getMetrics":       {true, true, true, true}, "getMetricsAlias": {true, true, true, true},
	"listTasks": {true, true, true, true}, "getTask": {true, true, true, true}, "listTaskAssignees": {true, true, true, true},
	"takeCustomerTask": {true, false, true, true}, "patchTask": {true, false, true, true}, "addTaskEvent": {true, false, true, true},
	"patchModelSettings": {false, true, true, true},
	"listAudit":          {false, true, true, true},
	"putSnapshot":        {false, true, true, true},
	"setSourcePolling":   {false, true, true, true},
	"patchService":       {false, true, true, true}, "patchAccount": {false, true, true, true},
	"emulatorTick": {false, true, true, true}, "emulatorActions": {false, true, true, true},
}

// Операции без сессии: вход, документация, спека, служебные пути.
var publicOperations = []string{
	"login", "getAuthConfig", "oidcLogin", "oidcCallback", "docs", "openapiYaml", "openapiJson",
	"healthz", "version", "ping", "pingHead",
}

func TestPermissionMatrixCoversEveryOperation(t *testing.T) {
	swagger, err := gen.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	inTable := map[string]string{}
	for pattern, op := range operations {
		inTable[op.id] = pattern
	}
	public := map[string]bool{}
	for _, id := range publicOperations {
		public[id] = true
	}

	seen := 0
	for path, item := range swagger.Paths.Map() {
		for method, op := range item.Operations() {
			// Встроенная спека хранит operationId с заглавной буквы.
			id := strings.ToLower(op.OperationID[:1]) + op.OperationID[1:]
			pattern, ok := inTable[id]
			if !ok {
				t.Errorf("%s %s (%s): нет в таблице operations — маршрут остался бы закрытым для всех", method, path, id)
				continue
			}
			if want := method + " " + path; pattern != want {
				t.Errorf("%s: шаблон %q, в контракте %q", id, pattern, want)
			}
			seen++
			if public[id] {
				for _, role := range []string{RoleMarketer, RoleAnalyst, RoleJury, RoleAdmin} {
					if !can(role, id) {
						t.Errorf("%s открыта без сессии, но can(%s) = false", id, role)
					}
				}
				continue
			}
			want, ok := wantAccess[id]
			if !ok {
				t.Errorf("%s: нет в ожидаемой матрице теста", id)
				continue
			}
			for role, allowed := range map[string]bool{RoleMarketer: want.marketer, RoleAnalyst: want.analyst, RoleJury: want.jury, RoleAdmin: want.admin} {
				if got := can(role, id); got != allowed {
					t.Errorf("can(%s, %s) = %v, ждали %v", role, id, got, allowed)
				}
			}
		}
	}
	if seen < 25 {
		t.Errorf("сверено операций %d, ждали не меньше 25", seen)
	}
}

func TestCanRejectsUnknown(t *testing.T) {
	if can("hacker", "listCustomers") || can("", "listCustomers") {
		t.Error("неизвестная роль получила доступ")
	}
	if can(RoleAdmin, "noSuchOperation") {
		t.Error("неизвестная операция разрешена")
	}
	if can(RoleMarketer, "patchModelSettings") {
		t.Error("маркетолог меняет модель")
	}
}

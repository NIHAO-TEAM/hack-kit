package auth

// Роли совпадают со значениями enum Role в контракте.
const (
	RoleMarketer = "marketer"
	RoleAnalyst  = "analyst"
	RoleJury     = "jury"
	RoleAdmin    = "admin"
)

// access — кому открыта операция.
type access int

const (
	// accessPublic — без сессии.
	accessPublic access = iota
	// accessAnyRole — любая роль с сессией.
	accessAnyRole
	// accessStaff — аналитик, jury, администратор: операции, которые меняют модель и данные.
	accessStaff
	// accessWork — работа с клиентом: маркетолог, jury, администратор. Аналитик только читает.
	accessWork
	// accessService — только служебный токен сборщика метрик; ни одна роль с сессией не допущена.
	accessService
)

// operations — таблица «шаблон маршрута ServeMux → операция». Маршрут, которого здесь нет,
// закрыт для всех без сессии и недоступен ни одной роли.
var operations = map[string]struct {
	id     string
	access access
}{
	"GET /healthz":                         {"healthz", accessPublic},
	"GET /version":                         {"version", accessPublic},
	"GET /ping":                            {"ping", accessPublic},
	"HEAD /ping":                           {"pingHead", accessPublic},
	"GET /api/v1/docs":                     {"docs", accessPublic},
	"GET /api/v1/docs/assets/":             {"docsAssets", accessPublic},
	"GET /api/v1/openapi.yml":              {"openapiYaml", accessPublic},
	"GET /api/v1/openapi.json":             {"openapiJson", accessPublic},
	"GET /{$}":                             {"front", accessPublic},
	"GET /":                                {"frontSPA", accessPublic},
	"POST /api/v1/auth/login":              {"login", accessPublic},
	"GET /api/v1/auth/config":              {"getAuthConfig", accessPublic},
	"GET /api/v1/auth/oidc/login":          {"oidcLogin", accessPublic},
	"GET /api/v1/auth/oidc/callback":       {"oidcCallback", accessPublic},
	"GET /api/v1/auth/me":                  {"getMe", accessAnyRole},
	"POST /api/v1/auth/logout":             {"logout", accessAnyRole},
	"GET /api/v1/customers":                {"listCustomers", accessAnyRole},
	"GET /api/v1/customers/export.csv":     {"exportCustomers", accessAnyRole},
	"GET /api/v1/customers/{id}":           {"getCustomer", accessAnyRole},
	"POST /api/v1/customers/{id}/task":     {"takeCustomerTask", accessWork},
	"GET /api/v1/tasks":                    {"listTasks", accessAnyRole},
	"GET /api/v1/tasks/assignees":          {"listTaskAssignees", accessAnyRole},
	"GET /api/v1/tasks/{taskId}":           {"getTask", accessAnyRole},
	"PATCH /api/v1/tasks/{taskId}":         {"patchTask", accessWork},
	"POST /api/v1/tasks/{taskId}/events":   {"addTaskEvent", accessWork},
	"GET /api/v1/analytics/summary":        {"getSummary", accessAnyRole},
	"GET /api/v1/analytics/churn-chart":    {"getChurnChart", accessAnyRole},
	"GET /api/v1/model/settings":           {"getModelSettings", accessAnyRole},
	"PATCH /api/v1/model/settings":         {"patchModelSettings", accessStaff},
	"GET /api/v1/model/sensitivity":        {"getModelSensitivity", accessStaff},
	"PUT /api/v1/source/snapshot":          {"putSnapshot", accessStaff},
	"GET /api/v1/source/status":            {"getSourceStatus", accessAnyRole},
	"PUT /api/v1/source/polling":           {"setSourcePolling", accessStaff},
	"PATCH /api/v1/source/services/{id}":   {"patchService", accessStaff},
	"PATCH /api/v1/source/accounts/{id}":   {"patchAccount", accessStaff},
	"POST /api/v1/source/emulator/tick":    {"emulatorTick", accessStaff},
	"POST /api/v1/source/emulator/actions": {"emulatorActions", accessStaff},
	"POST /api/v1/source/emulator/stream":  {"startEmulatorStream", accessStaff},
	"GET /api/v1/source/emulator/stream":   {"getEmulatorStream", accessStaff},
	"GET /api/v1/audit":                    {"listAudit", accessStaff},
	"GET /api/v1/events":                   {"getEvents", accessAnyRole},
	"GET /metrics":                         {"getMetrics", accessAnyRole},
	"GET /metics":                          {"getMetricsAlias", accessAnyRole},
	"GET /debug/pprof/":                    {"pprof", accessService},
}

// can решает, может ли роль выполнить операцию с сессией. Публичные операции доступны всем.
func can(role, operationID string) bool {
	for _, op := range operations {
		if op.id != operationID {
			continue
		}
		switch op.access {
		case accessPublic:
			return true
		case accessAnyRole:
			return isRole(role)
		case accessStaff:
			return role == RoleAnalyst || role == RoleJury || role == RoleAdmin
		case accessWork:
			return role == RoleMarketer || role == RoleJury || role == RoleAdmin
		}
	}
	return false
}

func isRole(role string) bool {
	switch role {
	case RoleMarketer, RoleAnalyst, RoleJury, RoleAdmin:
		return true
	}
	return false
}

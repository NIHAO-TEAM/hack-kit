// Package audit — журнал действий: вход и выход, правки настроек и данных. Модули пишут в него
// через Recorder; читает журнал GET /api/v1/audit.
package audit

import (
	"context"
	"time"
)

// Действия журнала; значения совпадают с enum AuditEntry.action в контракте.
const (
	ActionLogin          = "login"
	ActionLoginFailed    = "loginFailed"
	ActionLogout         = "logout"
	ActionSettingsChange = "settingsChanged"
	ActionSnapshotLoaded = "snapshotLoaded"
	ActionServiceChange  = "serviceChanged"
	ActionAccountChange  = "accountChanged"
	ActionEmulatorTick   = "emulatorTick"
	ActionEmulatorAction = "emulatorActions"
)

// Change — одна правка значения: было → стало, текстом.
type Change struct {
	Field string `json:"field"`
	From  string `json:"from"`
	To    string `json:"to"`
}

// Entry — запись журнала. Пароли, токены и email клиентов в неё не попадают: вызывающий
// передаёт только то, что можно показать аналитику.
type Entry struct {
	ID      int64
	At      time.Time
	Actor   string
	Action  string
	Summary string
	Changes []Change
}

// Recorder принимает записи. Нулевой *Service ничего не пишет, поэтому модули вызывают его без проверок.
type Recorder interface {
	Record(ctx context.Context, e Entry)
}

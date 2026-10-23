// Package source — снимок исходных данных, правки и эмулятор биллинга. Пока заглушка:
// каждая операция отвечает 501. Наполняет изменение snapshot-ingest.
package source

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"example.com/app/internal/audit"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
	"example.com/app/internal/platform/events"
	"example.com/app/internal/platform/httpx"
)

// SourceAPI обслуживает пути /source/*.
type SourceAPI struct {
	pool     *pgxpool.Pool
	bus      *events.Bus
	clk      clock.Clock
	maxBytes int64
	adapter  *Adapter
	audit    audit.Recorder
	checks   func(version int64) []gen.QualityCheck
}

// WithAudit подключает журнал действий: загрузка снимка, правки и шаги эмулятора.
func (a *SourceAPI) WithAudit(r audit.Recorder) *SourceAPI {
	a.audit = r
	return a
}

func (a *SourceAPI) record(r *http.Request, action, summary string, changes []audit.Change) {
	if a.audit != nil {
		a.audit.Record(r.Context(), audit.Entry{Action: action, Summary: summary, Changes: changes})
	}
}

// WithChecks подключает источник проверок качества данных: по версии снимка он отдаёт результаты
// или nil, пока пересчёт по этой версии не готов. Модуль не знает ядра модели, поэтому сборка сервера
// передаёт функцию снаружи.
func (a *SourceAPI) WithChecks(f func(version int64) []gen.QualityCheck) *SourceAPI {
	a.checks = f
	return a
}

// defaultMaxBytes — лимит тела снимка по умолчанию (SNAPSHOT_MAX_BYTES), 512 МБ.
const defaultMaxBytes = 512 << 20

// New создаёт обработчик модуля; пул, шину и часы создаёт cmd/api.
func New(pool *pgxpool.Pool, bus *events.Bus, clk clock.Clock) *SourceAPI {
	max := int64(defaultMaxBytes)
	if v, err := strconv.ParseInt(os.Getenv("SNAPSHOT_MAX_BYTES"), 10, 64); err == nil && v > 0 {
		max = v
	}
	return &SourceAPI{pool: pool, bus: bus, clk: clk, maxBytes: max}
}

// WithAdapter подключает опрос REST-биллинга: ручная загрузка ставит его на паузу, статус его показывает.
func (a *SourceAPI) WithAdapter(ad *Adapter) *SourceAPI {
	a.adapter = ad
	return a
}

// PutSnapshot — PUT /api/v1/source/snapshot. Права проверяет auth.Middleware.
func (a *SourceAPI) PutSnapshot(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, a.maxBytes)
	s, errs, err := ParseSnapshot(r.Body)
	if err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "тело не разобрано: "+err.Error())
		return
	}
	errs = append(errs, Validate(s)...)
	if len(errs) > 0 {
		if len(errs) > maxErrors {
			errs = errs[:maxErrors]
		}
		fe := make([]httpx.FieldError, len(errs))
		for i, e := range errs {
			fe[i] = httpx.FieldError{Field: e.Field, Message: e.Message}
		}
		httpx.WriteProblem(w, r, http.StatusUnprocessableEntity, "снимок не прошёл проверку", fe...)
		return
	}
	// Запись и пауза под одним замком с опросом: опрос не затрёт ручной снимок.
	emulatorMu.Lock()
	defer emulatorMu.Unlock()
	now := a.clk.Now()
	acc, err := replace(r.Context(), a.pool, s, ModelTime(s, now), now, OriginAPI)
	if err != nil {
		log.Printf("source: запись снимка: %v", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "снимок не записан, прежние данные сохранены")
		return
	}
	if a.adapter != nil {
		a.adapter.Pause() // ручной снимок главнее опроса, пока его не включат на экране «Источник»
	}
	a.bus.Publish(TopicChanged, acc.Version)
	a.record(r, audit.ActionSnapshotLoaded,
		fmt.Sprintf("Загружен снимок данных, версия %d: счетов %d, услуг %d, операций %d", acc.Version, acc.Accounts, acc.Services, acc.Transactions), nil)
	writeJSON(w, gen.SnapshotAccepted{
		Version: acc.Version, AsOf: acc.AsOf,
		Accounts: acc.Accounts, Services: acc.Services, Transactions: acc.Transactions,
	})
}

// SetSourcePolling — PUT /api/v1/source/polling. 404, если BILLING_URL не задан.
func (a *SourceAPI) SetSourcePolling(w http.ResponseWriter, r *http.Request) {
	if a.adapter == nil {
		httpx.WriteProblem(w, r, http.StatusNotFound, "опрос биллинга не настроен (BILLING_URL пуст)")
		return
	}
	var req gen.SourcePollingRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Paused {
		a.adapter.Pause()
	} else {
		a.adapter.Resume()
	}
	writeJSON(w, pollingBody(a.adapter.Status()))
}

func pollingBody(st PollStatus) *gen.SourcePolling {
	out := &gen.SourcePolling{
		Paused: st.Paused, IntervalSeconds: int(st.Interval.Seconds()),
		LastOkAt: st.LastOKAt, ErrorsTotal: st.ErrorsTotal,
	}
	if st.LastError != "" {
		out.LastError = &st.LastError
	}
	return out
}

// GetSourceStatus — GET /api/v1/source/status. Без снимков: версия 0.
func (a *SourceAPI) GetSourceStatus(w http.ResponseWriter, r *http.Request) {
	st, ok, err := currentStatus(r.Context(), a.pool)
	if err != nil {
		log.Printf("source: статус: %v", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "статус недоступен")
		return
	}
	now := a.clk.Now()
	out := gen.SourceStatus{LoadedAt: now, Checks: []gen.QualityCheck{}}
	if a.adapter != nil {
		out.Polling = pollingBody(a.adapter.Status())
	}
	if ok {
		src := gen.SourceStatusSource(st.Source)
		out.Version, out.Source, out.AsOf, out.LoadedAt = st.Version, &src, st.AsOf, st.LoadedAt
		out.AgeSeconds = int64(now.Sub(st.LoadedAt).Seconds())
		if a.checks != nil {
			if cs := a.checks(st.Version); cs != nil {
				out.Checks = cs
			}
		}
	}
	writeJSON(w, out)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("source: ответ: %v", err)
	}
}

// maxTickHours — предел одного сдвига времени: год.
const maxTickHours = 365 * 24

// emulatorMu не даёт двум шагам эмулятора читать один и тот же снимок одновременно.
var emulatorMu sync.Mutex

// EmulatorTick — POST /api/v1/source/emulator/tick. Права проверяет auth.Middleware.
func (a *SourceAPI) EmulatorTick(w http.ResponseWriter, r *http.Request) {
	var req gen.EmulatorTickRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(&req); err != nil {
			httpx.WriteProblem(w, r, http.StatusBadRequest, "тело не разобрано: "+err.Error())
			return
		}
	}
	hours := 24
	switch {
	case req.Hours != nil:
		hours = *req.Hours
	case req.Days != nil:
		hours = *req.Days * 24
	}
	if hours < 1 || hours > maxTickHours {
		httpx.WriteProblem(w, r, http.StatusBadRequest, fmt.Sprintf("сдвиг должен быть от 1 до %d часов", maxTickHours))
		return
	}
	if a.emulate(w, r, func(s *Snapshot) (*Snapshot, string, error) {
		next := Advance(*s, hours)
		return &next, OriginEmulator, nil
	}) {
		a.record(r, audit.ActionEmulatorTick, fmt.Sprintf("Эмулятор биллинга: время сдвинуто на %d ч", hours), nil)
	}
}

// EmulatorActions — POST /api/v1/source/emulator/actions.
func (a *SourceAPI) EmulatorActions(w http.ResponseWriter, r *http.Request) {
	var req gen.EmulatorActionsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "тело не разобрано: "+err.Error())
		return
	}
	if len(req.Actions) == 0 {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "нет действий")
		return
	}
	if a.emulate(w, r, func(s *Snapshot) (*Snapshot, string, error) {
		cur, origin := s, OriginEmulator
		for _, act := range req.Actions {
			if act.ActionId == ActionReset {
				fresh, err := resetSnapshot()
				if err != nil {
					return nil, "", err
				}
				fresh.AsOf = &seedAsOf
				cur, origin = fresh, OriginSeed
				continue
			}
			if err := applyAction(cur, act); err != nil {
				return nil, "", badRequest{err}
			}
			origin = OriginEmulator
		}
		return cur, origin, nil
	}) {
		ids := make([]string, len(req.Actions))
		for i, act := range req.Actions {
			ids[i] = string(act.ActionId)
		}
		a.record(r, audit.ActionEmulatorAction, "Эмулятор биллинга: действия "+strings.Join(ids, ", "), nil)
	}
}

type badRequest struct{ error }

type notFound struct{ error }

// emulate читает снимок, применяет шаг и пишет результат тем же путём, что PUT /source/snapshot:
// событие source.changed запускает пересчёт и обновление экранов.
// Возвращает true, если шаг записан.
func (a *SourceAPI) emulate(w http.ResponseWriter, r *http.Request, step func(*Snapshot) (*Snapshot, string, error)) bool {
	acc, _, ok := a.commit(w, r, step)
	if !ok {
		return false
	}
	w.Header().Set("X-Billing-Emulator", "simulated") // «имитация биллинга»
	writeJSON(w, gen.SnapshotAccepted{
		Version: acc.Version, AsOf: acc.AsOf,
		Accounts: acc.Accounts, Services: acc.Services, Transactions: acc.Transactions,
	})
	return true
}

// commit выполняет шаг над текущим снимком и записывает результат. При ошибке сам отвечает
// problem+json и возвращает ok=false.
func (a *SourceAPI) commit(w http.ResponseWriter, r *http.Request, step func(*Snapshot) (*Snapshot, string, error)) (acc Accepted, next *Snapshot, ok bool) {
	emulatorMu.Lock()
	defer emulatorMu.Unlock()

	cur, _, err := NewReader(a.pool).Load(r.Context())
	if err != nil && !errors.Is(err, ErrNoSnapshot) {
		log.Printf("source: эмулятор, чтение: %v", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "снимок не прочитан")
		return
	}
	if errors.Is(err, ErrNoSnapshot) {
		// Пустая база: эмулятор стартует с сида, как после reset.
		fresh, rerr := resetSnapshot()
		if rerr != nil {
			log.Printf("source: эмулятор, сид: %v", rerr)
			httpx.WriteProblem(w, r, http.StatusInternalServerError, "сид не прочитан")
			return
		}
		fresh.AsOf = &seedAsOf
		cur = *fresh
	}
	next, origin, err := step(&cur)
	var br badRequest
	var nf notFound
	if errors.As(err, &br) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, br.Error())
		return
	}
	if errors.As(err, &nf) {
		httpx.WriteProblem(w, r, http.StatusNotFound, nf.Error())
		return
	}
	if err != nil {
		log.Printf("source: эмулятор: %v", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "шаг эмулятора не выполнен")
		return
	}
	now := a.clk.Now()
	acc, err = replace(r.Context(), a.pool, next, ModelTime(next, now), now, origin)
	if err != nil {
		log.Printf("source: эмулятор, запись: %v", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "снимок не записан, прежние данные сохранены")
		return
	}
	a.bus.Publish(TopicChanged, acc.Version)
	return acc, next, true
}

// PatchService — PATCH /api/v1/source/services/{id}. Меняет одну услугу и пишет новый снимок
// тем же путём, что PUT: одна транзакция, событие source.changed.
func (a *SourceAPI) PatchService(w http.ResponseWriter, r *http.Request, id gen.ServiceId) {
	var req gen.ServicePatch
	if !decodeBody(w, r, &req) {
		return
	}
	var out, before Service
	_, _, ok := a.commit(w, r, func(s *Snapshot) (*Snapshot, string, error) {
		before = findService(s, int64(id))
		v, err := patchService(s, int64(id), req)
		out = v
		return s, OriginAPI, err
	})
	if !ok {
		return
	}
	a.record(r, audit.ActionServiceChange, fmt.Sprintf("Правка услуги %d", out.ID), serviceChanges(before, out))
	writeJSON(w, gen.Service{
		Id: out.ID, Name: out.Name, OwnerId: out.OwnerID, PaymentTerm: gen.PaymentTerm(out.PaymentTerm),
		AutoProlong: out.AutoProlong, CreatedAt: out.CreatedAt, ExpiresAt: out.ExpiresAt,
		Status: gen.ServiceStatus(out.Status), Price: out.Price,
	})
}

// PatchAccount — PATCH /api/v1/source/accounts/{id}.
func (a *SourceAPI) PatchAccount(w http.ResponseWriter, r *http.Request, id gen.AccountId) {
	var req gen.AccountPatch
	if !decodeBody(w, r, &req) {
		return
	}
	var out, before Account
	_, _, ok := a.commit(w, r, func(s *Snapshot) (*Snapshot, string, error) {
		before = findAccount(s, int64(id))
		v, err := patchAccount(s, int64(id), req)
		out = v
		return s, OriginAPI, err
	})
	if !ok {
		return
	}
	a.record(r, audit.ActionAccountChange, fmt.Sprintf("Правка счёта %d", out.ID), accountChanges(before, out))
	writeJSON(w, gen.Account{
		Id: out.ID, Email: out.Email, Balance: out.Balance, WithdrawBalance: out.WithdrawBalance,
		TotalReplenished: out.TotalReplenished, BonusBalance: out.BonusBalance,
		BonusState: gen.AccountBonusState(out.BonusState),
	})
}

// decodeBody разбирает JSON-тело правки; при ошибке отвечает 400 и возвращает false.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<16)).Decode(v); err != nil {
		httpx.WriteProblem(w, r, http.StatusBadRequest, "тело не разобрано: "+err.Error())
		return false
	}
	return true
}

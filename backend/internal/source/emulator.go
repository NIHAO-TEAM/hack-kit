package source

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/httpx"
)

// Действия эмулятора (actionId в EmulatorAction).
const (
	ActionTopUp           = "top_up"
	ActionEnableAutoprolong = "enable_autoprolong"
	ActionReset           = "reset"
)

// maxTickHours — предел одного сдвига времени: год.
const maxTickHours = 365 * 24

// prolongWaitLimit — сколько услуга живёт в prolong_wait до удаления.
const prolongWaitLimit = 5 * 24 * time.Hour

// emulatorMu не даёт двум шагам эмулятора читать один и тот же снимок одновременно.
var emulatorMu sync.Mutex

// termDuration — длина оплаченного срока.
func termDuration(term string) time.Duration {
	switch term {
	case "hour":
		return time.Hour
	case "week":
		return 7 * 24 * time.Hour
	case "month":
		return 30 * 24 * time.Hour
	case "quarter_year":
		return 90 * 24 * time.Hour
	case "half_year":
		return 180 * 24 * time.Hour
	case "year":
		return 365 * 24 * time.Hour
	}
	return 30 * 24 * time.Hour
}

// Advance двигает снимок на hours часов и возвращает новый; исходный не меняется.
// Активная услуга с истёкшим сроком продлевается и списывает цену с баланса владельца,
// если включено автопродление и денег хватает (почасовая — каждый час); иначе уходит
// в prolong_wait. prolong_wait старше 5 дней от конца срока становится deleted.
// Чистая функция: часов и ввода-вывода нет.
func Advance(s Snapshot, hours int) Snapshot {
	asOf := time.Time{}
	if s.AsOf != nil {
		asOf = s.AsOf.UTC()
	}
	end := asOf.Add(time.Duration(hours) * time.Hour)

	out := Snapshot{AsOf: &end}
	out.Accounts = append([]Account(nil), s.Accounts...)
	out.Services = append([]Service(nil), s.Services...)
	out.Transactions = append([]Transaction(nil), s.Transactions...)

	byID := make(map[int64]*Account, len(out.Accounts))
	for i := range out.Accounts {
		byID[out.Accounts[i].ID] = &out.Accounts[i]
	}
	for i := range out.Services {
		v := &out.Services[i]
		if v.Status == "active" {
			step := termDuration(v.PaymentTerm)
			for v.Status == "active" && !v.ExpiresAt.After(end) {
				acc := byID[v.OwnerID]
				if !v.AutoProlong || acc == nil || acc.Balance < v.Price {
					v.Status = "prolong_wait"
					break
				}
				acc.Balance -= v.Price
				acc.WithdrawBalance += v.Price
				v.ExpiresAt = v.ExpiresAt.Add(step)
			}
		}
		if v.Status == "prolong_wait" && !v.ExpiresAt.Add(prolongWaitLimit).After(end) {
			v.Status = "deleted"
		}
	}
	return out
}

// applyAction применяет одно действие к снимку на месте. reset сюда не приходит.
func applyAction(s *Snapshot, a gen.EmulatorAction) error {
	switch a.ActionId {
	case ActionTopUp:
		if a.Cents == nil || *a.Cents <= 0 {
			return errors.New("top_up: нужна сумма cents больше нуля")
		}
		for i := range s.Accounts {
			if s.Accounts[i].ID == a.CustomerId {
				s.Accounts[i].Balance += *a.Cents
				s.Accounts[i].TotalReplenished += *a.Cents
				return nil
			}
		}
	case ActionEnableAutoprolong:
		found := false
		for i := range s.Services {
			if s.Services[i].OwnerID == a.CustomerId {
				found = true
				if s.Services[i].Status != "deleted" {
					s.Services[i].AutoProlong = true
				}
			}
		}
		if found {
			return nil
		}
	default:
		return fmt.Errorf("неизвестное действие %q", a.ActionId)
	}
	return fmt.Errorf("%s: клиент %d не найден", a.ActionId, a.CustomerId)
}

// resetSnapshot — снимок сида: организаторы и, при SEED_STAND=true, стенд.
func resetSnapshot() (*Snapshot, error) {
	body, err := seedBody()
	if err != nil {
		return nil, err
	}
	s, errs, err := ParseSnapshot(bytes.NewReader(body))
	if err != nil || len(errs) > 0 {
		return nil, fmt.Errorf("source: вшитый сид не разбирается: %v %v", err, errs)
	}
	return s, nil
}

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
	a.emulate(w, r, func(s *Snapshot) (*Snapshot, string, error) {
		next := Advance(*s, hours)
		return &next, OriginAPI, nil
	})
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
	a.emulate(w, r, func(s *Snapshot) (*Snapshot, string, error) {
		cur, origin := s, OriginAPI
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
		}
		return cur, origin, nil
	})
}

type badRequest struct{ error }

// emulate читает снимок, применяет шаг и пишет результат тем же путём, что PUT /source/snapshot:
// событие source.changed запускает пересчёт и обновление экранов.
func (a *SourceAPI) emulate(w http.ResponseWriter, r *http.Request, step func(*Snapshot) (*Snapshot, string, error)) {
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
	if errors.As(err, &br) {
		httpx.WriteProblem(w, r, http.StatusBadRequest, br.Error())
		return
	}
	if err != nil {
		log.Printf("source: эмулятор: %v", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "шаг эмулятора не выполнен")
		return
	}
	now := a.clk.Now()
	acc, err := replace(r.Context(), a.pool, next, ModelTime(next, now), now, origin)
	if err != nil {
		log.Printf("source: эмулятор, запись: %v", err)
		httpx.WriteProblem(w, r, http.StatusInternalServerError, "снимок не записан, прежние данные сохранены")
		return
	}
	a.bus.Publish(TopicChanged, acc.Version)
	w.Header().Set("X-Billing-Emulator", "simulated") // «имитация биллинга»
	writeJSON(w, gen.SnapshotAccepted{
		Version: acc.Version, AsOf: acc.AsOf,
		Accounts: acc.Accounts, Services: acc.Services, Transactions: acc.Transactions,
	})
}

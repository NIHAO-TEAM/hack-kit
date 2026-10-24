package source

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

const organizersPath = "../../../cases/analizator/data.json"

func parseFile(t *testing.T, body []byte) *Snapshot {
	t.Helper()
	s, errs, err := ParseSnapshot(bytes.NewReader(body))
	if err != nil || len(errs) > 0 {
		t.Fatalf("разбор: err=%v errs=%v", err, errs)
	}
	return s
}

func TestParseOrganizers(t *testing.T) {
	body, err := os.ReadFile(organizersPath)
	if err != nil {
		t.Fatal(err)
	}
	s := parseFile(t, body)
	if len(s.Accounts) != 10 || len(s.Services) != 30 || len(s.Transactions) != 100 {
		t.Fatalf("счётчики %d/%d/%d", len(s.Accounts), len(s.Services), len(s.Transactions))
	}
	if errs := Validate(s); len(errs) > 0 {
		t.Fatalf("набор организаторов не прошёл проверки: %v", errs)
	}
}

func TestEmbeddedSeedMatchesCase(t *testing.T) {
	body, err := os.ReadFile(organizersPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, organizers) {
		t.Fatal("seed/organizers.json расходится с cases/analizator/data.json")
	}
}

func TestParseTypos(t *testing.T) {
	body, err := os.ReadFile(organizersPath)
	if err != nil {
		t.Fatal(err)
	}
	typo := string(body)
	for _, r := range [][2]string{
		{`"price"`, `"pice"`}, {`"paymentTerm"`, `"paymentTem"`}, {`"autoProlong"`, `"autoprolong"`},
		{`"quarter_year"`, `"quater_year"`}, {`"not_performed"`, `"not_pefomed"`}, {`"performed"`, `"pefomed"`},
	} {
		if !strings.Contains(typo, r[0]) {
			t.Fatalf("в data.json нет %s", r[0])
		}
		typo = strings.ReplaceAll(typo, r[0], r[1])
	}
	if !reflect.DeepEqual(parseFile(t, body), parseFile(t, []byte(typo))) {
		t.Fatal("снимок с опечатками кейса разобран иначе")
	}
}

func TestParseErrors(t *testing.T) {
	_, errs, err := ParseSnapshot(strings.NewReader(`{"accounts":[{"id":1}],"services":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	var fields []string
	for _, e := range errs {
		fields = append(fields, e.Field)
	}
	for _, want := range []string{"accounts[0].email", "transactions"} {
		found := false
		for _, f := range fields {
			found = found || f == want
		}
		if !found {
			t.Errorf("нет ошибки для %s: %v", want, fields)
		}
	}
	if _, _, err := ParseSnapshot(strings.NewReader(`[1]`)); err == nil {
		t.Error("массив вместо объекта принят")
	}
	if _, _, err := ParseSnapshot(strings.NewReader(`{"accounts":`)); err == nil {
		t.Error("обрезанное тело принято")
	}
}

func TestValidate(t *testing.T) {
	t0 := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Hour)
	good := func() *Snapshot {
		return &Snapshot{
			Accounts:     []Account{{ID: 1, Email: "a@x", BonusState: "locked"}},
			Services:     []Service{{ID: 1, Name: "svc-1", OwnerID: 1, PaymentTerm: "month", Status: "active", CreatedAt: t0, ExpiresAt: t1}},
			Transactions: []Transaction{{ID: 1, Amount: 10, Status: "not_performed", Type: "order", ServiceID: 1, CreatedAt: t0}},
		}
	}
	if errs := Validate(good()); len(errs) != 0 {
		t.Fatalf("верный снимок отклонён: %v", errs)
	}
	cases := []struct {
		name  string
		edit  func(*Snapshot)
		field string
	}{
		{"чужой ownerId", func(s *Snapshot) { s.Services[0].OwnerID = 99 }, "services[0].ownerId"},
		{"неизвестный статус услуги", func(s *Snapshot) { s.Services[0].Status = "zombie" }, "services[0].status"},
		{"отрицательная цена", func(s *Snapshot) { s.Services[0].Price = -1 }, "services[0].price"},
		{"отрицательный баланс", func(s *Snapshot) { s.Accounts[0].Balance = -5 }, "accounts[0].balance"},
		{"performedAt у not_performed", func(s *Snapshot) { s.Transactions[0].PerformedAt = &t1 }, "transactions[0].performedAt"},
		{"performed без даты", func(s *Snapshot) { s.Transactions[0].Status = "performed" }, "transactions[0].performedAt"},
		{"чужая услуга в транзакции", func(s *Snapshot) { s.Transactions[0].ServiceID = 7 }, "transactions[0].serviceId"},
		{"сумма вне диапазона", func(s *Snapshot) { s.Transactions[0].Amount = 1001 }, "transactions[0].amount"},
		{"отрицательная сумма транзакции", func(s *Snapshot) { s.Transactions[0].Amount = -1 }, "transactions[0].amount"},
		{"повтор email", func(s *Snapshot) { s.Accounts = append(s.Accounts, Account{ID: 2, Email: "a@x", BonusState: "locked"}) }, "accounts[1].email"},
		{"повтор id аккаунта", func(s *Snapshot) { s.Accounts = append(s.Accounts, s.Accounts[0]) }, "accounts[1].id"},
		{"expiresAt раньше createdAt", func(s *Snapshot) { s.Services[0].ExpiresAt = t0.Add(-time.Hour) }, "services[0].expiresAt"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := good()
			c.edit(s)
			errs := Validate(s)
			for _, e := range errs {
				if e.Field == c.field {
					return
				}
			}
			t.Fatalf("нет ошибки для %s: %v", c.field, errs)
		})
	}
}

func TestValidateCapsErrors(t *testing.T) {
	s := &Snapshot{}
	for i := 0; i < 300; i++ {
		s.Accounts = append(s.Accounts, Account{ID: 1})
	}
	if n := len(Validate(s)); n != maxErrors {
		t.Fatalf("ошибок %d, ждали %d", n, maxErrors)
	}
}

func TestModelTime(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	body, err := os.ReadFile(organizersPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ModelTime(parseFile(t, body), now), time.Date(2026, 10, 19, 14, 35, 53, 0, time.UTC); !got.Equal(want) {
		t.Errorf("без asOf: %v, ждали %v", got, want)
	}
	asOf := time.Date(2026, 10, 19, 15, 0, 0, 0, time.UTC)
	if got := ModelTime(&Snapshot{AsOf: &asOf}, now); !got.Equal(asOf) {
		t.Errorf("asOf из тела: %v", got)
	}
	if got := ModelTime(&Snapshot{}, now); !got.Equal(now) {
		t.Errorf("пустой снимок: %v", got)
	}
}

// Находка fuzz-проверки: цена вроде MaxInt64 переполняла платежи на горизонт и давала
// отрицательные потери. Теперь суммы выше предела — ошибка поля.
func TestValidateMoneyLimit(t *testing.T) {
	for _, price := range []string{"9223372036854775807", "1000000001"} {
		body := `{"accounts":[{"id":1,"email":"a@x.test","balance":1,"withdrawBalance":0,"totalReplenished":0,"bonusBalance":0,"bonusState":"locked"}],` +
			`"services":[{"id":1,"name":"a","ownerId":1,"paymentTerm":"hour","autoProlong":true,"createdAt":"2026-01-01T00:00:00Z","expiresAt":"2026-10-19T00:00:00Z","status":"active","price":` + price + `}],"transactions":[]}`
		s := parseFile(t, []byte(body))
		errs := Validate(s)
		if len(errs) != 1 || errs[0].Field != "services[0].price" {
			t.Errorf("цена %s: ошибки %v", price, errs)
		}
	}
	for _, bal := range []string{"1000000001", "9223372036854775807"} {
		body := `{"accounts":[{"id":1,"email":"a@x.test","balance":` + bal + `,"withdrawBalance":0,"totalReplenished":0,"bonusBalance":0,"bonusState":"locked"}],"services":[],"transactions":[]}`
		errs := Validate(parseFile(t, []byte(body)))
		if len(errs) != 1 || errs[0].Field != "accounts[0].balance" {
			t.Errorf("баланс %s: ошибки %v", bal, errs)
		}
	}
	edge := `{"accounts":[{"id":1,"email":"a@x.test","balance":1000000000,"withdrawBalance":0,"totalReplenished":0,"bonusBalance":0,"bonusState":"locked"}],"services":[],"transactions":[]}`
	if errs := Validate(parseFile(t, []byte(edge))); len(errs) != 0 {
		t.Errorf("сумма ровно на пределе отклонена: %v", errs)
	}
}

// recorder запоминает порядок массивов и число записей, которые отдал parseStream.
type recorder struct {
	order  []table
	counts [3]int
}

func (r *recorder) begin(t table) error { r.order = append(r.order, t); return nil }
func (r *recorder) end(table) error     { return nil }
func (r *recorder) account(int, Account) error {
	r.counts[tableAccounts]++
	return nil
}
func (r *recorder) service(int, Service) error {
	r.counts[tableServices]++
	return nil
}
func (r *recorder) transaction(int, Transaction) error {
	r.counts[tableTransactions]++
	return nil
}

// Три массива и asOf принимаются в любом порядке ключей верхнего уровня.
func TestParseStreamAnyKeyOrder(t *testing.T) {
	parts := map[string]string{
		"asOf":         `"asOf":"2026-10-19T15:00:00Z"`,
		"accounts":     `"accounts":[{"id":1,"email":"a@x","balance":0,"withdrawBalance":0,"totalReplenished":0,"bonusBalance":0,"bonusState":"locked"}]`,
		"services":     `"services":[{"id":1,"name":"vps","ownerId":1,"paymentTerm":"month","autoProlong":true,"createdAt":"2026-01-01T00:00:00Z","expiresAt":"2026-02-01T00:00:00Z","status":"active","price":100}]`,
		"transactions": `"transactions":[{"id":1,"amount":5,"status":"not_performed","createdAt":"2026-01-01T00:00:00Z","type":"order","serviceId":1}]`,
	}
	keys := []string{"asOf", "accounts", "services", "transactions"}
	var perm func(k int)
	n := 0
	try := func(order []string) {
		body := "{"
		for i, k := range order {
			if i > 0 {
				body += ","
			}
			body += parts[k]
		}
		body += "}"
		rec := &recorder{}
		asOf, errs, err := parseStream(strings.NewReader(body), rec, 0)
		if err != nil || len(errs) > 0 || asOf == nil {
			t.Fatalf("порядок %v: asOf=%v errs=%v err=%v", order, asOf, errs, err)
		}
		if rec.counts != [3]int{1, 1, 1} {
			t.Fatalf("порядок %v: записей %v", order, rec.counts)
		}
		var want []table
		for _, k := range order {
			for ti, name := range tableKeys {
				if k == name {
					want = append(want, table(ti))
				}
			}
		}
		if fmt.Sprint(rec.order) != fmt.Sprint(want) {
			t.Fatalf("порядок %v: массивы пришли как %v", order, rec.order)
		}
		n++
	}
	perm = func(k int) {
		if k == len(keys) {
			try(append([]string(nil), keys...))
			return
		}
		for i := k; i < len(keys); i++ {
			keys[k], keys[i] = keys[i], keys[k]
			perm(k + 1)
			keys[k], keys[i] = keys[i], keys[k]
		}
	}
	perm(0)
	if n != 24 {
		t.Fatalf("проверено порядков %d, ждали 24", n)
	}
}

// Ссылка на массив, который в теле ещё не встретился, проверяется после его конца.
func TestValidatorDeferredRefs(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	svc := func(id, owner int64) Service {
		return Service{ID: id, Name: "vps", OwnerID: owner, PaymentTerm: "month", Status: "active", CreatedAt: now, ExpiresAt: now}
	}
	acc := Account{ID: 1, Email: "a@x", BonusState: "locked"}

	v := newValidator(0)
	v.service(0, svc(1, 1)) // услуга раньше аккаунтов
	v.service(1, svc(2, 9)) // владельца нет вовсе
	v.account(0, acc)
	v.endTable(tableAccounts)
	v.endTable(tableServices)
	errs := v.finish()
	if len(errs) != 1 || errs[0].Field != "services[1].ownerId" {
		t.Fatalf("ждали одну ошибку services[1].ownerId, получили %v", errs)
	}

	// Массив services не пришёл: платёж ссылается в пустоту.
	v = newValidator(0)
	v.account(0, acc)
	v.endTable(tableAccounts)
	v.transaction(0, Transaction{ID: 1, Status: "not_performed", Type: "order", ServiceID: 5, CreatedAt: now})
	errs = v.finish()
	if len(errs) != 1 || errs[0].Field != "transactions[0].serviceId" {
		t.Fatalf("ждали ошибку transactions[0].serviceId, получили %v", errs)
	}
}

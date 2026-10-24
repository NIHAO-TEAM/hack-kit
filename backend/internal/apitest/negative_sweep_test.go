package apitest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"example.com/app/internal/apitest"
)

// Проход «кривой ввод → не 5xx» по всем операциям контракта (ADR-0004, п. 6).

type sweepOp struct {
	id, method, path string
	op               *openapi3.Operation
	params           openapi3.Parameters
}

func loadSweepOps(t testing.TB) []sweepOp {
	t.Helper()
	raw, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := openapi3.NewLoader().LoadFromData(raw)
	if err != nil {
		t.Fatal(err)
	}
	var ops []sweepOp
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			params := append(openapi3.Parameters{}, item.Parameters...)
			params = append(params, op.Parameters...)
			ops = append(ops, sweepOp{id: op.OperationID, method: method, path: path, op: op, params: params})
		}
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].id < ops[j].id })
	return ops
}

// badScalars — значения для параметров пути и запроса независимо от схемы.
func badScalars(s *openapi3.Schema) []string {
	vals := []string{
		"", "abc", "-1", "0", "1.5", "NaN", "1e999", "99999999999999999999", "-99999999999999999999",
		"00000000-0000-0000-0000-000000000000", "not-a-uuid", "' OR 1=1 --", "<script>", "../../etc/passwd",
		"%00", "я", strings.Repeat("x", 10000), "null", "true", "[]", "{}",
	}
	if s != nil {
		if s.Min != nil {
			vals = append(vals, fmt.Sprint(*s.Min-1))
		}
		if s.Max != nil {
			vals = append(vals, fmt.Sprint(*s.Max+1))
		}
		if len(s.Enum) > 0 {
			vals = append(vals, "zzz-not-in-enum")
		}
	}
	return vals
}

func schemaOf(p *openapi3.Parameter) *openapi3.Schema {
	if p.Schema == nil {
		return nil
	}
	return p.Schema.Value
}

// badBodies строит кривые тела по схеме запроса.
func badBodies(op *openapi3.Operation) [][]byte {
	bodies := [][]byte{
		nil, []byte(""), []byte("{"), []byte("null"), []byte("[]"), []byte(`"str"`), []byte("123"), []byte("{}"),
		[]byte(`{"unknown_field":1}`), []byte("\xff\xfe\x00"), bytes.Repeat([]byte("["), 100000),
		append(append([]byte(`{"pad":"`), bytes.Repeat([]byte("x"), 12<<20)...), `"}`...),
	}
	if op.RequestBody == nil || op.RequestBody.Value == nil {
		return bodies
	}
	mt := op.RequestBody.Value.Content.Get("application/json")
	if mt == nil || mt.Schema == nil || mt.Schema.Value == nil {
		return bodies
	}
	wrong := []any{"x", 123456789012345, -1, 1.5, true, nil, []any{}, map[string]any{}, strings.Repeat("я", 100000), 1e308}
	names := make([]string, 0, len(mt.Schema.Value.Properties))
	for name := range mt.Schema.Value.Properties {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, w := range wrong {
			b, err := json.Marshal(map[string]any{name: w})
			if err == nil {
				bodies = append(bodies, b)
			}
		}
	}
	// все обязательные поля неверного типа разом
	if req := mt.Schema.Value.Required; len(req) > 0 {
		for _, w := range wrong {
			m := map[string]any{}
			for _, name := range req {
				m[name] = w
			}
			if b, err := json.Marshal(m); err == nil {
				bodies = append(bodies, b)
			}
		}
	}
	return bodies
}

// sweepRequest — один кривой запрос.
type sweepRequest struct {
	method, path string
	body         []byte
}

func (o sweepOp) requests() []sweepRequest {
	var pathParams, queryParams []*openapi3.Parameter
	for _, p := range o.params {
		switch p.Value.In {
		case "path":
			pathParams = append(pathParams, p.Value)
		case "query":
			queryParams = append(queryParams, p.Value)
		}
	}
	fill := func(vals map[string]string) string {
		path := o.path
		for _, p := range pathParams {
			v, ok := vals[p.Name]
			if !ok {
				v = "1"
			}
			path = strings.ReplaceAll(path, "{"+p.Name+"}", url.PathEscape(v))
		}
		return path
	}
	var reqs []sweepRequest
	add := func(path string, body []byte) {
		reqs = append(reqs, sweepRequest{method: strings.ToUpper(o.method), path: path, body: body})
	}
	// чистый запрос с пустым телом и с каждым кривым телом
	base := fill(nil)
	add(base, nil)
	hasBody := o.method == http.MethodPost || o.method == http.MethodPut || o.method == http.MethodPatch
	if hasBody {
		for _, b := range badBodies(o.op) {
			add(base, b)
		}
	}
	for _, p := range pathParams {
		for _, v := range badScalars(schemaOf(p)) {
			add(fill(map[string]string{p.Name: v}), nil)
		}
	}
	for _, p := range queryParams {
		for _, v := range badScalars(schemaOf(p)) {
			add(base+"?"+url.QueryEscape(p.Name)+"="+url.QueryEscape(v), nil)
		}
		// повтор параметра и массив
		add(base+"?"+url.QueryEscape(p.Name)+"=1&"+url.QueryEscape(p.Name)+"=2", nil)
	}
	add(base+"?unknown=1&limit=-5&offset=abc", nil)
	return reqs
}

type sweepSubject struct {
	name   string
	header string // значение Authorization
}

func sweepSubjects(t testing.TB, s *apitest.Stand) []sweepSubject {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"login": "jury", "password": apitest.Password("jury")})
	resp, err := http.Post(s.URL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.Token == "" {
		t.Fatalf("вход под jury: %d %v", resp.StatusCode, err)
	}
	return []sweepSubject{
		{name: "anonymous"},
		{name: "jury", header: "Bearer " + out.Token},
		{name: "source", header: "Bearer " + apitest.SourceToken},
		{name: "metrics", header: "Bearer " + apitest.MetricsToken},
	}
}

// undeclared — пары «операция код», которых нет в контракте: не провал, а список для правки контракта.
var undeclared sync.Map

var sweepClient = &http.Client{Timeout: 20 * time.Second}

// declared — код ответа описан у операции в контракте.
func declared(o sweepOp, code int) bool {
	if o.op == nil || o.op.Responses == nil {
		return true
	}
	return o.op.Responses.Status(code) != nil || o.op.Responses.Default() != nil
}

// checkSweep шлёт запрос и возвращает описание нарушения или пустую строку.
func checkSweep(s *apitest.Stand, o sweepOp, sub sweepSubject, r sweepRequest) string {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, r.method, s.URL+r.path, bytes.NewReader(r.body))
	if err != nil {
		return "" // запрос невозможно собрать на клиенте — до сервера не дойдёт
	}
	if r.body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if sub.header != "" {
		req.Header.Set("Authorization", sub.header)
	}
	resp, err := sweepClient.Do(req)
	if err != nil {
		return fmt.Sprintf("нет ответа: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 400 {
		return ""
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 500 && (resp.StatusCode != http.StatusNotImplemented || !declared(o, resp.StatusCode)) {
		return fmt.Sprintf("статус %d: %.200s", resp.StatusCode, body)
	}
	if !declared(o, resp.StatusCode) {
		undeclared.Store(fmt.Sprintf("%s %d", o.id, resp.StatusCode), true)
	}
	mt, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if mt != "application/problem+json" {
		return fmt.Sprintf("код %d с Content-Type %q, ждали application/problem+json", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	var p struct {
		CorrelationID string `json:"correlation_id"`
		Status        int    `json:"status"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return fmt.Sprintf("код %d, тело не JSON: %.200s", resp.StatusCode, body)
	}
	if p.CorrelationID == "" || p.CorrelationID != resp.Header.Get("X-Request-Id") {
		return fmt.Sprintf("код %d: correlation_id %q не равен X-Request-Id %q", resp.StatusCode, p.CorrelationID, resp.Header.Get("X-Request-Id"))
	}
	if p.Status != resp.StatusCode {
		return fmt.Sprintf("код %d: status в теле %d", resp.StatusCode, p.Status)
	}
	return ""
}

// sweepStand — стенд с воркером пересчёта и загруженным снимком, чтобы ручки данных отвечали по существу.
func sweepStand(t testing.TB) *apitest.Stand {
	t.Helper()
	s := apitest.New(t)
	s.StartRecalc(t)
	snap, err := os.ReadFile("../../../cases/analizator/data.json")
	if err != nil {
		t.Fatal(err)
	}
	if res, err := s.WithBearer(t, apitest.SourceToken).PutSnapshotWithBodyWithResponse(context.Background(), "application/json", bytes.NewReader(snap)); err != nil || res.StatusCode() != http.StatusOK {
		t.Fatalf("снимок: %v %v", err, res)
	}
	return s
}

func TestNegativeSweep(t *testing.T) {
	ops := loadSweepOps(t)
	raw, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if want := bytes.Count(raw, []byte("operationId:")); len(ops) != want {
		t.Fatalf("проход видит операций: %d, в контракте operationId: %d", len(ops), want)
	}
	s := sweepStand(t)
	subjects := sweepSubjects(t, s)

	total := 0
	for _, o := range ops {
		reqs := o.requests()
		t.Run(o.id, func(t *testing.T) {
			bad := 0
			for _, sub := range subjects {
				for _, r := range reqs {
					total++
					if msg := checkSweep(s, o, sub, r); msg != "" {
						bad++
						if bad <= 10 {
							t.Errorf("%s %s [%s] тело %.60q: %s", r.method, r.path[:min(len(r.path), 120)], sub.name, r.body, msg)
						}
					}
				}
			}
		})
	}
	t.Logf("операций: %d, запросов: %d", len(ops), total)
	var codes []string
	undeclared.Range(func(k, _ any) bool { codes = append(codes, k.(string)); return true })
	sort.Strings(codes)
	t.Logf("коды, которых нет в контракте (%d): %s", len(codes), strings.Join(codes, ", "))
}

// FuzzAPI — тот же генератор, случайные значения. Короткий прогон: go test -fuzz FuzzAPI -fuzztime 30s.
func FuzzAPI(f *testing.F) {
	ops := loadSweepOps(f)
	s := sweepStand(f)
	subjects := sweepSubjects(f, s)
	for i := range ops {
		f.Add(uint8(i), uint8(1), "1", "limit=-1", []byte(`{}`))
		f.Add(uint8(i), uint8(0), "abc", "offset=x", []byte(`{"a":`))
	}
	f.Fuzz(func(t *testing.T, opIdx, subIdx uint8, pathVal, query string, body []byte) {
		o := ops[int(opIdx)%len(ops)]
		sub := subjects[int(subIdx)%len(subjects)]
		path := o.path
		for _, p := range o.params {
			if p.Value.In == "path" {
				path = strings.ReplaceAll(path, "{"+p.Value.Name+"}", url.PathEscape(pathVal))
			}
		}
		if query != "" {
			path += "?" + url.PathEscape(query)
		}
		var b []byte
		if o.method == http.MethodPost || o.method == http.MethodPut || o.method == http.MethodPatch {
			b = body
		}
		if msg := checkSweep(s, o, sub, sweepRequest{method: strings.ToUpper(o.method), path: path, body: b}); msg != "" {
			t.Errorf("%s %s [%s]: %s", o.method, path, sub.name, msg)
		}
	})
}

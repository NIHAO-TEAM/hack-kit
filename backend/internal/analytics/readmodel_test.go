package analytics_test

import (
	"cmp"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"example.com/app/internal/analytics"
	"example.com/app/internal/platform/api/gen"
	"example.com/app/internal/platform/clock"
)

func listAPI(pf *analytics.Portfolio) *analytics.AnalyticsAPI {
	return analytics.New(nil, nil, clock.NewFake(asOf)).WithFeed(fakeFeed{pf})
}

type listed struct {
	Items      []gen.CustomerRow `json:"items"`
	Total      int               `json:"total"`
	Version    int64             `json:"version"`
	NextCursor *string           `json:"nextCursor"`
}

func list(t testing.TB, api *analytics.AnalyticsAPI, p gen.ListCustomersParams) (listed, int) {
	t.Helper()
	rec := httptest.NewRecorder()
	api.ListCustomers(rec, httptest.NewRequest(http.MethodGet, "/api/v1/customers", nil), p)
	var out listed
	if rec.Code == 200 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
	}
	return out, rec.Code
}

func ids(rows []gen.CustomerRow) []int64 {
	out := make([]int64, len(rows))
	for i, r := range rows {
		out[i] = r.Id
	}
	return out
}

// tiedPortfolio — клиенты с повторами значений и email в разном регистре, порядок по id нарушен:
// проверяет вторичный ключ id и закрепление «нет оценки» и «потерян» внизу.
func tiedPortfolio(n int) *analytics.Portfolio {
	levels := []gen.RiskLevel{gen.High, gen.Medium, gen.Low, gen.Unscored, gen.Lost}
	cs := make([]analytics.Customer, n)
	for i := range cs {
		id := int64((i*7919)%n + 1) // перестановка 1..n
		lvl := levels[i%len(levels)]
		row := gen.CustomerRow{
			Id: id, Level: lvl, Balance: int64(i % 5), TotalReplenished: int64(i % 3),
			Email: []string{"A", "b", "B", "a"}[i%4] + string(rune('a'+i%7)) + "@example.test",
		}
		if lvl != gen.Unscored && lvl != gen.Lost {
			s := float64(i % 11)
			row.Score = &s
			row.ExpectedLossCents = int64(i % 13)
		}
		cs[i] = analytics.Customer{Row: row}
	}
	return &analytics.Portfolio{Version: 4, Customers: cs}
}

// reference — прежняя сортировка на запрос, как была до модели чтения.
func reference(pf *analytics.Portfolio, level *gen.RiskLevel, sortBy gen.ListCustomersParamsSort, desc bool) []int64 {
	var rows []gen.CustomerRow
	for _, c := range pf.Customers {
		if level == nil || c.Row.Level == *level {
			rows = append(rows, c.Row)
		}
	}
	scored := func(r gen.CustomerRow) bool { return r.Level != gen.Unscored && r.Level != gen.Lost }
	key := func(x, y gen.CustomerRow) int {
		f := func(p *float64) float64 {
			if p == nil {
				return 0
			}
			return *p
		}
		switch sortBy {
		case "id":
			return cmp.Compare(x.Id, y.Id)
		case "email":
			return cmp.Compare(strings.ToLower(x.Email), strings.ToLower(y.Email))
		case "balance":
			return cmp.Compare(x.Balance, y.Balance)
		case "totalReplenished":
			return cmp.Compare(x.TotalReplenished, y.TotalReplenished)
		case "score":
			return cmp.Compare(f(x.Score), f(y.Score))
		}
		return cmp.Compare(x.ExpectedLossCents, y.ExpectedLossCents)
	}
	pin := sortBy == "score" || sortBy == "expectedLoss"
	slices.SortStableFunc(rows, func(x, y gen.CustomerRow) int {
		if pin {
			switch xs, ys := scored(x), scored(y); {
			case xs != ys:
				if xs {
					return -1
				}
				return 1
			case !xs:
				return cmp.Compare(x.Id, y.Id)
			}
		}
		c := key(x, y)
		if desc {
			c = -c
		}
		if c != 0 {
			return c
		}
		return cmp.Compare(x.Id, y.Id)
	})
	return ids(rows)
}

// Порядок модели чтения совпадает с прежней сортировкой на запрос для каждого поля, направления и фильтра.
func TestReadModelOrderMatchesReference(t *testing.T) {
	pf := tiedPortfolio(500)
	api := listAPI(pf)
	high := gen.High
	for _, sortBy := range []gen.ListCustomersParamsSort{"id", "email", "balance", "totalReplenished", "score", "expectedLoss"} {
		for _, order := range []gen.ListCustomersParamsOrder{gen.Asc, gen.Desc} {
			for _, level := range []*gen.RiskLevel{nil, &high} {
				got, code := list(t, api, gen.ListCustomersParams{Sort: &sortBy, Order: &order, Level: level, Limit: ptr(200)})
				if code != 200 {
					t.Fatalf("%s %s: статус %d", sortBy, order, code)
				}
				want := reference(pf, level, sortBy, order == gen.Desc)
				if !slices.Equal(ids(got.Items), want[:len(got.Items)]) || got.Total != len(want) {
					t.Errorf("%s %s level=%v: порядок или total не совпали", sortBy, order, level != nil)
				}
			}
		}
	}
}

func ptr[T any](v T) *T { return &v }

// Обход курсором 10k клиентов: без повторов и пропусков, порядок как у страниц offset.
func TestCursorWalksAllWithoutGapsOrRepeats(t *testing.T) {
	pf := tiedPortfolio(10_000)
	api := listAPI(pf)
	for _, sortBy := range []gen.ListCustomersParamsSort{"expectedLoss", "email"} {
		want := reference(pf, nil, sortBy, true) // порядок по умолчанию — по убыванию
		var seen []int64
		var cur *string
		for pages := 0; ; pages++ {
			if pages > 100 {
				t.Fatal("курсор не кончается")
			}
			got, code := list(t, api, gen.ListCustomersParams{Sort: &sortBy, Limit: ptr(150), Cursor: cur})
			if code != 200 {
				t.Fatalf("статус %d", code)
			}
			seen = append(seen, ids(got.Items)...)
			if cur = got.NextCursor; cur == nil {
				break
			}
			if len(got.Items) != 150 {
				t.Fatalf("курсор есть, а страница неполная: %d", len(got.Items))
			}
		}
		if !slices.Equal(seen, want) {
			t.Errorf("%s: обход курсором (%d строк) не совпал с порядком (%d)", sortBy, len(seen), len(want))
		}
	}
}

func TestCursorStaleAndMisuse(t *testing.T) {
	pf := tiedPortfolio(300)
	api := listAPI(pf)
	first, _ := list(t, api, gen.ListCustomersParams{Limit: ptr(10)})
	if first.NextCursor == nil {
		t.Fatal("курсора нет")
	}
	// Страница без хвоста курсора не даёт.
	if last, _ := list(t, api, gen.ListCustomersParams{Limit: ptr(200), Offset: ptr(200)}); last.NextCursor != nil {
		t.Error("у последней страницы есть nextCursor")
	}

	// Новый пересчёт — другая версия: курсор отвергается явно.
	newer := tiedPortfolio(300)
	newer.Version = 5
	if _, code := list(t, listAPI(newer), gen.ListCustomersParams{Cursor: first.NextCursor}); code != 409 {
		t.Errorf("курсор прежней версии: статус %d, ждали 409", code)
	}
	asc := gen.Asc
	cases := map[string]gen.ListCustomersParams{
		"с offset":          {Cursor: first.NextCursor, Offset: ptr(5)},
		"другая сортировка": {Cursor: first.NextCursor, Order: &asc},
		"мусор":             {Cursor: ptr("не-курсор")},
		"чужое поле":        {Cursor: ptr("NC5leHRlcm5hbC5hLjA")},
	}
	for name, p := range cases {
		if _, code := list(t, api, p); code != 400 {
			t.Errorf("%s: статус %d, ждали 400", name, code)
		}
	}
}

type swapFeed struct {
	p atomic.Pointer[analytics.Portfolio]
}

func (f *swapFeed) Portfolio() *analytics.Portfolio { return f.p.Load() }

// Читатели при смене портфеля видят либо старую модель целиком, либо новую (запуск с -race).
func TestReadersSeeWholeModelDuringSwap(t *testing.T) {
	small, large := tiedPortfolio(400), tiedPortfolio(900)
	small.Version, large.Version = 1, 2
	feed := &swapFeed{}
	feed.p.Store(small)
	api := analytics.New(nil, nil, clock.NewFake(asOf)).WithFeed(feed)
	sizes := map[int64]int{1: 400, 2: 900}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				got, code := list(t, api, gen.ListCustomersParams{Limit: ptr(200)})
				if code != 200 || got.Total != sizes[got.Version] || len(got.Items) != 200 {
					t.Errorf("статус %d, версия %d, total %d: модель не целая", code, got.Version, got.Total)
					return
				}
			}
		}()
	}
	for i := range 40 {
		next := small
		if i%2 == 0 {
			next = large
		}
		fresh := &analytics.Portfolio{Version: next.Version, Customers: next.Customers} // новый пересчёт — новая модель
		feed.p.Store(fresh)
	}
	close(stop)
	wg.Wait()
}

// Ответ из заранее собранных байтов совпадает побайтно с json.Encoder для gen.CustomerPage.
func TestListBodyMatchesEncoder(t *testing.T) {
	api := listAPI(tiedPortfolio(300))
	for _, limit := range []int{1, 50, 200} {
		rec := httptest.NewRecorder()
		api.ListCustomers(rec, httptest.NewRequest(http.MethodGet, "/api/v1/customers", nil), gen.ListCustomersParams{Limit: ptr(limit)})
		var page gen.CustomerPage
		if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		var want strings.Builder
		if err := json.NewEncoder(&want).Encode(page); err != nil {
			t.Fatal(err)
		}
		if rec.Body.String() != want.String() {
			t.Errorf("limit %d: тело ответа отличается от json.Encoder", limit)
		}
	}
}

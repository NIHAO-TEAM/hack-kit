package auth

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"example.com/app/internal/platform/clock"
)

const (
	sessionCacheShards  = 16
	sessionCacheMaxSize = 10000
)

type cacheEntry struct {
	id      Identity
	expires time.Time
	stored  time.Time
}

type cacheShard struct {
	mu sync.RWMutex
	m  map[string]cacheEntry
}

// sessionCache хранит личность по хэшу токена в памяти процесса. Запись живёт не дольше ttl
// и не дольше срока самой сессии; отрицательные ответы не хранятся.
type sessionCache struct {
	clk    clock.Clock
	ttl    time.Duration
	perMax int
	shards [sessionCacheShards]cacheShard

	// gen растёт при каждом удалении: загрузка, начатая до удаления, не кладёт в кэш устаревшее.
	gen          atomic.Uint64
	hits, misses atomic.Uint64
}

// newSessionCache создаёт кэш; ttl <= 0 выключает его (nil).
func newSessionCache(clk clock.Clock, ttl time.Duration, maxSize int) *sessionCache {
	if ttl <= 0 {
		return nil
	}
	c := &sessionCache{clk: clk, ttl: ttl, perMax: max(1, maxSize/sessionCacheShards)}
	for i := range c.shards {
		c.shards[i].m = map[string]cacheEntry{}
	}
	return c
}

// shard выбирает шард по первому байту хэша (hex-строки sha256).
func (c *sessionCache) shard(hash string) *cacheShard {
	var b byte
	if len(hash) >= 2 {
		b = hexNibble(hash[0])<<4 | hexNibble(hash[1])
	}
	return &c.shards[int(b)%sessionCacheShards]
}

func hexNibble(ch byte) byte {
	switch {
	case ch >= '0' && ch <= '9':
		return ch - '0'
	case ch >= 'a' && ch <= 'f':
		return ch - 'a' + 10
	}
	return 0
}

// get возвращает живую запись; счётчики попаданий и промахов ведёт сам.
func (c *sessionCache) get(hash string) (Identity, bool) {
	if c == nil {
		return Identity{}, false
	}
	sh := c.shard(hash)
	sh.mu.RLock()
	e, ok := sh.m[hash]
	sh.mu.RUnlock()
	if ok && c.clk.Now().Before(e.expires) {
		c.hits.Add(1)
		return e.id, true
	}
	c.misses.Add(1)
	return Identity{}, false
}

// generation возвращает счётчик удалений; его берут до чтения из БД и отдают в put.
func (c *sessionCache) generation() uint64 {
	if c == nil {
		return 0
	}
	return c.gen.Load()
}

// put кладёт запись, если с момента gen ничего не удаляли. При переполнении шарда вытесняет
// истёкшие, затем самые старые.
func (c *sessionCache) put(hash string, id Identity, sessionExpires time.Time, gen uint64) {
	if c == nil {
		return
	}
	now := c.clk.Now()
	exp := now.Add(c.ttl)
	if sessionExpires.Before(exp) {
		exp = sessionExpires
	}
	if !exp.After(now) {
		return
	}
	sh := c.shard(hash)
	sh.mu.Lock()
	defer sh.mu.Unlock()
	if c.gen.Load() != gen {
		return
	}
	if _, exists := sh.m[hash]; !exists && len(sh.m) >= c.perMax {
		evict(sh.m, now)
	}
	sh.m[hash] = cacheEntry{id: id, expires: exp, stored: now}
}

// evict убирает истёкшие записи, а если место не освободилось — самую старую.
func evict(m map[string]cacheEntry, now time.Time) {
	var oldest string
	var oldestAt time.Time
	freed := false
	for k, e := range m {
		if !now.Before(e.expires) {
			delete(m, k)
			freed = true
			continue
		}
		if oldest == "" || e.stored.Before(oldestAt) {
			oldest, oldestAt = k, e.stored
		}
	}
	if !freed && oldest != "" {
		delete(m, oldest)
	}
}

// delete убирает запись (выход, конец сессии).
func (c *sessionCache) delete(hash string) {
	if c == nil {
		return
	}
	sh := c.shard(hash)
	sh.mu.Lock()
	c.gen.Add(1)
	delete(sh.m, hash)
	sh.mu.Unlock()
}

// deleteUser убирает все записи пользователя: после смены роли она не живёт в кэше до конца TTL.
func (c *sessionCache) deleteUser(userID int64) {
	if c == nil {
		return
	}
	c.gen.Add(1)
	for i := range c.shards {
		sh := &c.shards[i]
		sh.mu.Lock()
		for k, e := range sh.m {
			if e.id.UserID == userID {
				delete(sh.m, k)
			}
		}
		sh.mu.Unlock()
	}
}

func (c *sessionCache) size() int {
	n := 0
	for i := range c.shards {
		c.shards[i].mu.RLock()
		n += len(c.shards[i].m)
		c.shards[i].mu.RUnlock()
	}
	return n
}

var descSessionCache = prometheus.NewDesc("churn_session_cache_total",
	"Проверки сессии по кэшу процесса (штуки); result: hit — без запроса в БД, miss — с запросом.", []string{"result"}, nil)

// Collector возвращает метрики кэша сессий для /metrics.
func (a *AuthAPI) Collector() prometheus.Collector { return cacheCollector{a.svc.cache} }

type cacheCollector struct{ c *sessionCache }

func (cacheCollector) Describe(ch chan<- *prometheus.Desc) { ch <- descSessionCache }

func (cc cacheCollector) Collect(ch chan<- prometheus.Metric) {
	var hit, miss uint64
	if cc.c != nil {
		hit, miss = cc.c.hits.Load(), cc.c.misses.Load()
	}
	ch <- prometheus.MustNewConstMetric(descSessionCache, prometheus.CounterValue, float64(hit), "hit")
	ch <- prometheus.MustNewConstMetric(descSessionCache, prometheus.CounterValue, float64(miss), "miss")
}

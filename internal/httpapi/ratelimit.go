package httpapi

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lizhicheng00/relay-controller/internal/core"
)

const (
	rateWindow      = time.Minute
	counterTTL      = 2 * rateWindow
	maxRateCounters = 10_000
)

type RateLimiter struct {
	enabled     bool
	limit       int
	mutex       sync.Mutex
	counters    map[string]windowCounter
	lastCleanup time.Time
	now         func() time.Time
}

type windowCounter struct {
	windowStart time.Time
	lastSeen    time.Time
	count       int
}

func NewRateLimiter(enabled bool, requestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		enabled: enabled, limit: requestsPerMinute, counters: make(map[string]windowCounter), now: time.Now,
	}
}

func (r *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if !isAPIPath(request.URL.Path) || r.Allow(rateKey(request)) {
			next.ServeHTTP(response, request)
			return
		}
		writeJSON(response, http.StatusTooManyRequests,
			core.NewError(http.StatusTooManyRequests, core.CodeRateLimited, "rate limited").Response())
	})
}

func (r *RateLimiter) Allow(key string) bool {
	if !r.enabled || r.limit <= 0 {
		return true
	}
	now := r.now()
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.lastCleanup.IsZero() || now.Sub(r.lastCleanup) >= rateWindow {
		for existingKey, counter := range r.counters {
			if !counter.lastSeen.IsZero() && now.Sub(counter.lastSeen) > counterTTL {
				delete(r.counters, existingKey)
			}
		}
		r.lastCleanup = now
	}
	counter, exists := r.counters[key]
	if !exists && len(r.counters) >= maxRateCounters {
		return false
	}
	if !exists || now.Sub(counter.windowStart) >= rateWindow {
		counter.windowStart = now
		counter.count = 0
	}
	counter.lastSeen = now
	counter.count++
	r.counters[key] = counter
	return counter.count <= r.limit
}

func rateKey(request *http.Request) string {
	namespace := request.Header.Get("X-Namespace")
	if core.ValidIdentifier(namespace) {
		return "namespace:" + namespace
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return "ip:" + host
	}
	return "ip:" + request.RemoteAddr
}

func isAPIPath(path string) bool {
	return path == apiBase+"/limits" || path == apiBase+"/tunnels" || strings.HasPrefix(path, apiBase+"/tunnels/")
}

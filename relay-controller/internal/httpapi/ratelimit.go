package httpapi

import (
	"net/http"
	"sync"
	"time"

	"relay-controller/internal/auth"
	"relay-controller/internal/core"
)

const (
	rateWindow      = time.Minute
	maxRateCounters = 10_000
)

type RateLimiter struct {
	limit       int
	mutex       sync.Mutex
	counters    map[string]windowCounter
	lastCleanup time.Time
	now         func() time.Time
}

type windowCounter struct {
	windowStart time.Time
	count       int
}

func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		limit: requestsPerMinute, counters: make(map[string]windowCounter), now: time.Now,
	}
}

func (r *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		principal, _ := auth.PrincipalFrom(request.Context())
		if r.Allow(principal.Namespace) {
			next.ServeHTTP(response, request)
			return
		}
		writeJSON(response, http.StatusTooManyRequests,
			core.NewError(http.StatusTooManyRequests, core.CodeRateLimited, "rate limited").Response())
	})
}

func (r *RateLimiter) Allow(key string) bool {
	now := r.now()
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.lastCleanup.IsZero() || now.Sub(r.lastCleanup) >= rateWindow {
		for existingKey, counter := range r.counters {
			if now.Sub(counter.windowStart) >= 2*rateWindow {
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
	counter.count++
	r.counters[key] = counter
	return counter.count <= r.limit
}

package middleware

import (
	"sync"
	"time"
)

type RateLimiter struct {
	mu      sync.Mutex
	entries map[string][]time.Time
	limit   int
	window  time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		entries: make(map[string][]time.Time),
		limit:   limit,
		window:  window,
	}
}

func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-r.window)
	filtered := r.entries[key][:0]
	for _, t := range r.entries[key] {
		if t.After(windowStart) {
			filtered = append(filtered, t)
		}
	}
	r.entries[key] = filtered
	if len(filtered) >= r.limit {
		return false
	}
	r.entries[key] = append(r.entries[key], now)
	return true
}

package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket implements a per-key token bucket rate limiter.
type TokenBucket struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64 // tokens per second
	capacity int64   // max burst
}

type bucket struct {
	tokens    float64
	lastCheck time.Time
}

// New creates a token bucket rate limiter.
func New(ratePerSecond float64, burstCapacity int64) *TokenBucket {
	return &TokenBucket{
		buckets:  make(map[string]*bucket),
		rate:     ratePerSecond,
		capacity: burstCapacity,
	}
}

// Allow checks if a request is allowed for the given key.
func (tb *TokenBucket) Allow(key string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	b, ok := tb.buckets[key]
	if !ok {
		b = &bucket{
			tokens:    float64(tb.capacity),
			lastCheck: time.Now(),
		}
		tb.buckets[key] = b
	}

	now := time.Now()
	elapsed := now.Sub(b.lastCheck).Seconds()
	b.tokens += elapsed * tb.rate
	if b.tokens > float64(tb.capacity) {
		b.tokens = float64(tb.capacity)
	}
	b.lastCheck = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

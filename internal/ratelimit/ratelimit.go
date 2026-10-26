package ratelimit

import (
	"sync"
	"time"
)

type Limiter struct {
	limit  int
	window time.Duration

	mu        sync.Mutex
	counters  map[string]*counter
	lastSweep time.Time
}

type counter struct {
	hits    int
	resetAt time.Time
}

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{
		limit:    limit,
		window:   window,
		counters: make(map[string]*counter),
	}
}

func (l *Limiter) Allow(key string, now time.Time) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.sweep(now)

	c, ok := l.counters[key]
	if !ok || !now.Before(c.resetAt) {
		c = &counter{resetAt: now.Add(l.window)}
		l.counters[key] = c
	}

	if c.hits >= l.limit {
		return false, c.resetAt.Sub(now)
	}
	c.hits++
	return true, 0
}

func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now

	for key, c := range l.counters {
		if !now.Before(c.resetAt) {
			delete(l.counters, key)
		}
	}
}

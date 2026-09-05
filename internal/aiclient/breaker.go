package aiclient

import (
	"sync"
	"time"
)

const (
	DefaultBreakerThreshold = 3
	DefaultBreakerCooldown  = 60 * time.Second
)

type Breaker struct {
	mu        sync.Mutex
	failures  int
	openedAt  time.Time
	probing   bool
	threshold int
	cooldown  time.Duration
	now       func() time.Time
}

func NewBreaker(threshold int, cooldown time.Duration) *Breaker {
	if threshold <= 0 {
		threshold = DefaultBreakerThreshold
	}
	if cooldown <= 0 {
		cooldown = DefaultBreakerCooldown
	}
	return &Breaker{threshold: threshold, cooldown: cooldown, now: time.Now}
}

func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.openedAt.IsZero() {
		return true
	}
	if b.now().Sub(b.openedAt) < b.cooldown {
		return false
	}
	if b.probing {
		return false
	}
	b.probing = true
	return true
}

func (b *Breaker) Succeed() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failures = 0
	b.probing = false
	b.openedAt = time.Time{}
}

func (b *Breaker) Fail() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.probing {
		b.probing = false
		b.openedAt = b.now()
		return
	}

	b.failures++
	if b.failures >= b.threshold {
		b.openedAt = b.now()
	}
}

func (b *Breaker) Open() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return !b.openedAt.IsZero() && b.now().Sub(b.openedAt) < b.cooldown
}

func NewBreakerWithClock(threshold int, cooldown time.Duration, now func() time.Time) *Breaker {
	breaker := NewBreaker(threshold, cooldown)
	breaker.now = now
	return breaker
}

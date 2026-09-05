package aiclient

import (
	"errors"
	"sync"
	"time"

	"terrion-backend/internal/constants"
)

var ErrBreakerOpen = errors.New("aiclient: circuit breaker terbuka")

type breaker struct {
	mu       sync.Mutex
	failures int
	openedAt time.Time
	now      func() time.Time
}

func newBreaker() *breaker {
	return &breaker{now: time.Now}
}

func (b *breaker) allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.failures < constants.AIBreakerTrip {
		return true
	}
	if b.now().Sub(b.openedAt) < constants.AIBreakerCooldown {
		return false
	}

	b.failures = constants.AIBreakerTrip - 1
	return true
}

func (b *breaker) succeed() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
}

func (b *breaker) fail() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failures++
	if b.failures >= constants.AIBreakerTrip {
		b.openedAt = b.now()
	}
}

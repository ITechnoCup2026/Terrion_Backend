package aiclient

import (
	"testing"
	"time"

	"terrion-backend/internal/constants"
)

func TestBreakerStaysClosedWhileCallsSucceed(t *testing.T) {
	b := newBreaker()

	for i := range 10 {
		if !b.allow() {
			t.Fatalf("breaker menolak panggilan ke-%d padahal semuanya sukses", i)
		}
		b.succeed()
	}
}

func TestBreakerOpensAfterThreeConsecutiveFailures(t *testing.T) {
	b := newBreaker()

	for i := range constants.AIBreakerTrip {
		if !b.allow() {
			t.Fatalf("breaker terbuka terlalu cepat, pada kegagalan ke-%d", i)
		}
		b.fail()
	}

	if b.allow() {
		t.Error("breaker masih meloloskan panggilan setelah tiga kegagalan")
	}
}

func TestBreakerForgetsFailuresAfterASuccess(t *testing.T) {
	b := newBreaker()

	b.allow()
	b.fail()
	b.allow()
	b.fail()
	b.allow()
	b.succeed()
	b.allow()
	b.fail()

	if !b.allow() {
		t.Error("dua kegagalan setelah sukses seharusnya belum membuka breaker")
	}
}

func TestBreakerHalfOpensAfterCooldown(t *testing.T) {
	sekarang := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)

	b := newBreaker()
	b.now = func() time.Time { return sekarang }

	for range constants.AIBreakerTrip {
		b.allow()
		b.fail()
	}
	if b.allow() {
		t.Fatal("breaker seharusnya terbuka")
	}

	sekarang = sekarang.Add(constants.AIBreakerCooldown + time.Second)

	if !b.allow() {
		t.Fatal("breaker seharusnya setengah terbuka setelah cooldown")
	}

	b.fail()
	if b.allow() {
		t.Error("kegagalan saat setengah terbuka seharusnya membuka lagi")
	}
}

package aiclient_test

import (
	"testing"
	"time"

	"terrion-backend/internal/aiclient"
)

func TestBreakerStartsClosed(t *testing.T) {
	breaker := aiclient.NewBreaker(3, time.Minute)

	if !breaker.Allow() || breaker.Open() {
		t.Fatal("breaker baru harus tertutup")
	}
}

func TestBreakerToleratesFailuresBelowTheThreshold(t *testing.T) {
	breaker := aiclient.NewBreaker(3, time.Minute)

	breaker.Fail()
	breaker.Fail()

	if !breaker.Allow() {
		t.Fatal("dua kegagalan belum boleh membuka breaker")
	}
}

func TestBreakerOpensOnConsecutiveFailures(t *testing.T) {
	breaker := aiclient.NewBreaker(3, time.Minute)

	for range 3 {
		breaker.Fail()
	}

	if breaker.Allow() {
		t.Fatal("tiga kegagalan berturut-turut harus membuka breaker")
	}
}

func TestASuccessResetsTheCounter(t *testing.T) {
	breaker := aiclient.NewBreaker(3, time.Minute)

	breaker.Fail()
	breaker.Fail()
	breaker.Succeed()
	breaker.Fail()
	breaker.Fail()

	if !breaker.Allow() {
		t.Fatal("keberhasilan harus menolkan penghitung kegagalan")
	}
}

func TestBreakerLetsOneRequestThroughAfterTheCooldown(t *testing.T) {
	clock := time.Now()
	breaker := aiclient.NewBreakerWithClock(3, time.Minute, func() time.Time { return clock })

	for range 3 {
		breaker.Fail()
	}
	clock = clock.Add(61 * time.Second)

	if !breaker.Allow() {
		t.Fatal("setelah jeda, satu permintaan harus diloloskan")
	}
	if breaker.Allow() {
		t.Fatal("hanya satu permintaan yang boleh lolos saat setengah terbuka")
	}
}

func TestAFailedProbeReopensTheBreaker(t *testing.T) {
	clock := time.Now()
	breaker := aiclient.NewBreakerWithClock(3, time.Minute, func() time.Time { return clock })

	for range 3 {
		breaker.Fail()
	}
	clock = clock.Add(61 * time.Second)
	breaker.Allow()
	breaker.Fail()

	if breaker.Allow() {
		t.Fatal("percobaan yang gagal harus menutup lagi selama satu jeda penuh")
	}
}

func TestASuccessfulProbeClosesTheBreaker(t *testing.T) {
	clock := time.Now()
	breaker := aiclient.NewBreakerWithClock(3, time.Minute, func() time.Time { return clock })

	for range 3 {
		breaker.Fail()
	}
	clock = clock.Add(61 * time.Second)
	breaker.Allow()
	breaker.Succeed()

	if !breaker.Allow() || breaker.Open() {
		t.Fatal("percobaan yang berhasil harus menutup breaker sepenuhnya")
	}
}

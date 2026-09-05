package aiclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"terrion-backend/internal/constants"
)

// Anggaran 3,5 detik di ARCHITECTURE.md §6.1 adalah anggaran untuk SELURUH
// panggilan, "termasuk 1 retry" — bukan jatah per percobaan. Diukur sebelum
// perbaikan ini, satu layanan AI yang menggantung menahan pengguna 7,35 detik
// dengan dua percobaan penuh, karena timeout dihitung ulang dari nol pada
// percobaan kedua dan tidak ada deadline di atasnya: controller mengoper
// ctx.UserContext() yang polos.

func klienDenganAnggaran(t *testing.T, anggaran time.Duration, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "token-uji", anggaran)
	if client == nil {
		t.Fatal("NewClient mengembalikan nil untuk URL yang terisi")
	}
	return client
}

func TestProposeSpendsOneBudgetForBothAttempts(t *testing.T) {
	const anggaran = 500 * time.Millisecond
	var percobaan int32

	client := klienDenganAnggaran(t, anggaran, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&percobaan, 1)
		// Menggantung sampai klien menyerah. Batas atasnya ada supaya
		// server uji bisa ditutup: di Windows konteks request sisi server
		// tidak selalu dibatalkan begitu klien memutus koneksi.
		select {
		case <-r.Context().Done():
		case <-time.After(3 * anggaran):
		}
	})

	mulai := time.Now()
	_, err := client.Propose(context.Background(), contohPermintaan())
	berlalu := time.Since(mulai)

	if err == nil {
		t.Fatal("Propose seharusnya gagal ketika layanan AI menggantung")
	}

	// Percobaan pertama sudah menghabiskan seluruh anggaran, jadi tidak ada
	// yang tersisa untuk yang kedua — dan itulah maksudnya.
	if got := atomic.LoadInt32(&percobaan); got != 1 {
		t.Errorf("layanan dihubungi %d kali; percobaan kedua memakai anggaran "+
			"yang sudah habis, dan mengulang seluruh komputasi Python", got)
	}
	if batas := anggaran + constants.AIRetryBackoff; berlalu > batas {
		t.Errorf("Propose menahan pengguna %v, di atas batas %v — anggaran %v "+
			"ditagihkan lebih dari sekali", berlalu, batas, anggaran)
	}
}

func TestProposeSkipsTheRetryWhenTheBudgetIsAlreadyGone(t *testing.T) {
	// Anggaran lebih pendek dari jeda backoff terpendek, jadi menunggu jeda
	// itu saja sudah melewati batas. Percobaan kedua tidak boleh berangkat.
	const anggaran = 100 * time.Millisecond
	var percobaan int32

	client := klienDenganAnggaran(t, anggaran, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&percobaan, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"solver_failed","message":"boom"}}`))
	})

	if _, err := client.Propose(context.Background(), contohPermintaan()); err == nil {
		t.Fatal("Propose seharusnya gagal ketika anggarannya habis")
	}

	if got := atomic.LoadInt32(&percobaan); got != 1 {
		t.Errorf("layanan dihubungi %d kali; jeda backoff %v sendirian sudah "+
			"melewati anggaran %v", got, constants.AIRetryBackoff, anggaran)
	}
}

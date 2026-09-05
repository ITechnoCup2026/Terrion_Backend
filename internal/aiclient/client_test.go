package aiclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func contohPermintaan() Request {
	return Request{
		ContractVersion: "1.0",
		RequestID:       "req-1",
		Seed:            1,
		Season:          Season{Label: "MT I 2026/2027", Start: "2026-10-01", End: "2027-03-31"},
		Objectives:      []string{"aman"},
		Candidates: []Candidate{{
			ID: "c001", PlotRef: "p1", AreaHa: 1, CommodityRef: "k1", VarietyRef: "v1",
			PlantingDate: "2026-10-05", HarvestStart: "2027-01-02", HarvestEnd: "2027-01-16",
			TonnesLow: 1, TonnesMid: 2, TonnesHigh: 3, Plausibility: "plausible",
		}},
	}
}

func contohRespons() Response {
	return Response{
		ContractVersion: "1.0",
		RequestID:       "req-1",
		Solver:          "cp-sat",
		SolverVersion:   "1.0.0",
		Plans: []PlanResult{{
			Objective:       "aman",
			CandidateIDs:    []string{"c001"},
			Metrics:         Metrics{PeakTonnesP50: 2, PeakTonnesP90: 3, TotalTonnes: 2},
			NarrativeSource: "template",
		}},
	}
}

func layani(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	client := NewClient(server.URL, "token-uji", 2*time.Second)
	if client == nil {
		t.Fatal("NewClient mengembalikan nil untuk URL yang terisi")
	}
	return client
}

func TestNewClientReturnsNilWhenTheURLIsEmpty(t *testing.T) {
	if NewClient("", "token", time.Second) != nil {
		t.Error("URL kosong seharusnya menghasilkan klien nil, yaitu 'tidak ada layanan AI'")
	}
	if NewClient("   ", "token", time.Second) != nil {
		t.Error("URL berisi spasi saja seharusnya juga menghasilkan nil")
	}
}

func TestProposeSendsTheBearerTokenAndDecodesTheResponse(t *testing.T) {
	var terlihat string

	client := layani(t, func(w http.ResponseWriter, r *http.Request) {
		terlihat = r.Header.Get("Authorization")
		if r.URL.Path != "/v1/plan/propose" {
			t.Errorf("jalur = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(contohRespons())
	})

	response, err := client.Propose(context.Background(), contohPermintaan())
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if terlihat != "Bearer token-uji" {
		t.Errorf("Authorization = %q", terlihat)
	}
	if len(response.Plans) != 1 || response.Plans[0].CandidateIDs[0] != "c001" {
		t.Errorf("respons tidak terurai sebagaimana mestinya: %+v", response.Plans)
	}
}

func TestProposeRetriesOnceOnAServerError(t *testing.T) {
	var panggilan int32

	client := layani(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&panggilan, 1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"code":"solver_failed","message":"boom"}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(contohRespons())
	})

	if _, err := client.Propose(context.Background(), contohPermintaan()); err != nil {
		t.Fatalf("Propose seharusnya berhasil pada percobaan kedua: %v", err)
	}
	if got := atomic.LoadInt32(&panggilan); got != 2 {
		t.Errorf("jumlah panggilan = %d, mau 2", got)
	}
}

func TestProposeDoesNotRetryOnAClientError(t *testing.T) {
	var panggilan int32

	client := layani(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&panggilan, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"malformed_request","message":"nope"}}`))
	})

	_, err := client.Propose(context.Background(), contohPermintaan())
	if err == nil {
		t.Fatal("Propose seharusnya gagal pada 400")
	}
	if got := atomic.LoadInt32(&panggilan); got != 1 {
		t.Errorf("400 di-retry %d kali; permintaan yang salah bentuk adalah bug kita, "+
			"mengulanginya hanya menggandakan bug", got-1)
	}

	var galat *ServiceError
	if !errors.As(err, &galat) || galat.Code != "malformed_request" {
		t.Errorf("galat = %v, mau ServiceError malformed_request", err)
	}
}

func TestProposeRejectsAMismatchedContractMajor(t *testing.T) {
	client := layani(t, func(w http.ResponseWriter, r *http.Request) {
		response := contohRespons()
		response.ContractVersion = "2.0"
		_ = json.NewEncoder(w).Encode(response)
	})

	_, err := client.Propose(context.Background(), contohPermintaan())
	if err == nil {
		t.Fatal("respons dengan MAJOR berbeda seharusnya ditolak, bukan dipakai setengah")
	}
}

func TestProposeStopsCallingOnceTheBreakerIsOpen(t *testing.T) {
	var panggilan int32

	client := layani(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&panggilan, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	for range 5 {
		_, _ = client.Propose(context.Background(), contohPermintaan())
	}

	if _, err := client.Propose(context.Background(), contohPermintaan()); !errors.Is(err, ErrBreakerOpen) {
		t.Errorf("galat = %v, mau ErrBreakerOpen", err)
	}

	sebelum := atomic.LoadInt32(&panggilan)
	_, _ = client.Propose(context.Background(), contohPermintaan())
	if atomic.LoadInt32(&panggilan) != sebelum {
		t.Error("breaker terbuka tetapi permintaan masih menyentuh jaringan")
	}
}

func TestFingerprintIgnoresTheRequestID(t *testing.T) {
	a := contohPermintaan()
	b := contohPermintaan()
	b.RequestID = "req-lain"

	if Fingerprint(a) != Fingerprint(b) {
		t.Error("sidik jari berubah karena request_id; cache tidak akan pernah kena")
	}

	c := contohPermintaan()
	c.Seed = 999
	if Fingerprint(a) == Fingerprint(c) {
		t.Error("seed berbeda seharusnya menghasilkan sidik jari berbeda")
	}
}

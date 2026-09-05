package aiclient

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func layananYangBerjalan(t *testing.T) *Client {
	t.Helper()

	baseURL := os.Getenv("TERRION_AI_URL")
	token := os.Getenv("TERRION_AI_TOKEN")
	if baseURL == "" || token == "" {
		t.Skip("lewati: set TERRION_AI_URL dan TERRION_AI_TOKEN untuk menguji layanan sungguhan")
	}

	client := NewClient(baseURL, token, 10*time.Second)
	if client == nil {
		t.Fatalf("NewClient(%q) = nil", baseURL)
	}
	return client
}

func permintaanEmas(t *testing.T) Request {
	t.Helper()

	emas, err := os.ReadFile("testdata/propose_request.golden.json")
	if err != nil {
		t.Fatalf("baca berkas emas: %v", err)
	}

	var request Request
	if err := json.Unmarshal(emas, &request); err != nil {
		t.Fatalf("urai berkas emas: %v", err)
	}
	return request
}

func TestAgainstARunningAiService(t *testing.T) {
	client := layananYangBerjalan(t)
	request := permintaanEmas(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := client.Propose(ctx, request)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	if response.RequestID != request.RequestID {
		t.Errorf("RequestID = %q, want %q", response.RequestID, request.RequestID)
	}

	if len(response.Plans) != len(request.Objectives) {
		t.Fatalf("Plans = %d, want %d", len(response.Plans), len(request.Objectives))
	}

	dikenal := make(map[string]bool, len(request.Candidates))
	for _, kandidat := range request.Candidates {
		dikenal[kandidat.ID] = true
	}

	for _, rencana := range response.Plans {
		if len(rencana.CandidateIDs) == 0 {
			t.Errorf("%s: candidate_ids kosong", rencana.Objective)
		}

		terpakai := make(map[string]bool, len(rencana.CandidateIDs))
		for _, id := range rencana.CandidateIDs {
			if !dikenal[id] {
				t.Errorf("%s: candidate_id %q tidak pernah dikirim Go", rencana.Objective, id)
			}
			if terpakai[id] {
				t.Errorf("%s: candidate_id %q dipakai dua kali", rencana.Objective, id)
			}
			terpakai[id] = true
		}

		if rencana.Metrics.PeakTonnesP90 < rencana.Metrics.PeakTonnesP50 {
			t.Errorf("%s: p90 %v lebih kecil dari p50 %v",
				rencana.Objective, rencana.Metrics.PeakTonnesP90, rencana.Metrics.PeakTonnesP50)
		}

		if rencana.Narrative == "" {
			t.Errorf("%s: narasi kosong", rencana.Objective)
		}

		if rencana.NarrativeSource != "llm" && rencana.NarrativeSource != "template" {
			t.Errorf("%s: narrative_source = %q, di luar kontrak", rencana.Objective, rencana.NarrativeSource)
		}
	}

	t.Logf("solver=%s elapsed_ms=%d degraded=%v",
		response.Solver, response.ElapsedMs, response.Diagnostics.Degraded)
}

func TestARunningAiServiceIsDeterministicAboutWhichCandidatesItPicks(t *testing.T) {
	client := layananYangBerjalan(t)
	request := permintaanEmas(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	pertama, err := client.Propose(ctx, request)
	if err != nil {
		t.Fatalf("Propose pertama: %v", err)
	}

	kedua, err := client.Propose(ctx, request)
	if err != nil {
		t.Fatalf("Propose kedua: %v", err)
	}

	for i := range pertama.Plans {
		awal, ulang := pertama.Plans[i], kedua.Plans[i]

		if awal.Objective != ulang.Objective {
			t.Fatalf("urutan objektif berubah: %q lalu %q", awal.Objective, ulang.Objective)
		}

		if len(awal.CandidateIDs) != len(ulang.CandidateIDs) {
			t.Errorf("%s: jumlah kandidat %d lalu %d",
				awal.Objective, len(awal.CandidateIDs), len(ulang.CandidateIDs))
			continue
		}

		for j := range awal.CandidateIDs {
			if awal.CandidateIDs[j] != ulang.CandidateIDs[j] {
				t.Errorf("%s: kandidat ke-%d %q lalu %q, padahal seed sama",
					awal.Objective, j, awal.CandidateIDs[j], ulang.CandidateIDs[j])
			}
		}

		if !metrikSama(awal.Metrics, ulang.Metrics) {
			t.Errorf("%s: metrik berubah antar dua permintaan identik: %+v lalu %+v",
				awal.Objective, awal.Metrics, ulang.Metrics)
		}
	}
}

func metrikSama(a, b Metrics) bool {
	if a.PeakTonnesP50 != b.PeakTonnesP50 || a.PeakTonnesP90 != b.PeakTonnesP90 {
		return false
	}
	if a.TotalTonnes != b.TotalTonnes || a.DemandCoveredKg != b.DemandCoveredKg {
		return false
	}
	if (a.GrossValue == nil) != (b.GrossValue == nil) {
		return false
	}
	return a.GrossValue == nil || *a.GrossValue == *b.GrossValue
}

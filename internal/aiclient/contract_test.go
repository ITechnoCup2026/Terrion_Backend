package aiclient

import (
	"encoding/json"
	"os"
	"testing"
)

func TestProposeRequestMatchesTheGoldenFile(t *testing.T) {
	harga := 5200.0
	kapasitas := 12.5

	request := Request{
		ContractVersion:       "1.0",
		RequestID:             "5a1f0c9e-3c2a-4b3e-9f5a-77c0e2b1d004",
		Seed:                  20260905,
		Season:                Season{Label: "MT I 2026/2027", Start: "2026-10-01", End: "2027-03-31"},
		Objectives:            []string{"aman", "pendapatan", "pasar"},
		CapacityTonnesPerWeek: &kapasitas,
		Candidates: []Candidate{
			{
				ID: "c001", PlotRef: "p1", AreaHa: 0.82,
				CommodityRef: "k1", VarietyRef: "v3",
				PlantingDate: "2026-10-05", HarvestStart: "2027-01-02", HarvestEnd: "2027-01-16",
				TonnesLow: 2.91, TonnesMid: 3.6, TonnesHigh: 4.42,
				Plausibility: "plausible", PricePerKg: &harga,
			},
			{
				ID: "c002", PlotRef: "p1", AreaHa: 0.82,
				CommodityRef: "k1", VarietyRef: "v3",
				PlantingDate: "2026-10-12", HarvestStart: "2027-01-09", HarvestEnd: "2027-01-23",
				TonnesLow: 2.88, TonnesMid: 3.57, TonnesHigh: 4.39,
				Plausibility: "plausible", PricePerKg: &harga,
			},
		},
		Demand: []DemandRow{
			{CommodityRef: "k1", ISOWeek: "2027-01-04", Kg: 4000},
		},
	}

	dihasilkan, err := json.MarshalIndent(request, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	emas, err := os.ReadFile("testdata/propose_request.golden.json")
	if err != nil {
		t.Fatalf("baca berkas emas: %v", err)
	}

	if string(dihasilkan) != string(normalise(emas)) {
		t.Errorf("permintaan menyimpang dari kontrak v1.0.\n\ndihasilkan:\n%s\n\nemas:\n%s\n\n"+
			"Kalau perubahan ini disengaja: naikkan contract_version, perbarui berkas emas "+
			"ini, lalu salin ke Terrion_AI/tests/fixtures/ pada commit yang sama.",
			dihasilkan, emas)
	}
}

func TestProposeResponseParsesFromTheGoldenFile(t *testing.T) {
	emas, err := os.ReadFile("testdata/propose_response.golden.json")
	if err != nil {
		t.Fatalf("baca berkas emas: %v", err)
	}

	var response Response
	if err := json.Unmarshal(emas, &response); err != nil {
		t.Fatalf("urai respons emas: %v", err)
	}

	if len(response.Plans) != 3 {
		t.Fatalf("len(Plans) = %d, mau 3", len(response.Plans))
	}
	if response.Plans[0].Objective != "aman" {
		t.Errorf("rencana pertama = %q, mau aman", response.Plans[0].Objective)
	}
	if len(response.Plans[0].CandidateIDs) == 0 {
		t.Error("rencana aman tidak memuat satu pun kandidat")
	}
	if response.Plans[0].Metrics.PeakTonnesP90 <= 0 {
		t.Error("PeakTonnesP90 tidak terisi")
	}
}

func normalise(raw []byte) []byte {
	out := make([]byte, 0, len(raw))
	for _, b := range raw {
		if b != '\r' {
			out = append(out, b)
		}
	}
	if n := len(out); n > 0 && out[n-1] == '\n' {
		out = out[:n-1]
	}
	return out
}

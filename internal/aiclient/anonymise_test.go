package aiclient

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRefsAreAssignedInOrderAndReused(t *testing.T) {
	table := NewRefTable()

	if got := table.Plot("11111111-1111-4111-8111-111111111111"); got != "p1" {
		t.Errorf("lahan pertama = %q, mau p1", got)
	}
	if got := table.Plot("22222222-2222-4222-8222-222222222222"); got != "p2" {
		t.Errorf("lahan kedua = %q, mau p2", got)
	}
	if got := table.Plot("11111111-1111-4111-8111-111111111111"); got != "p1" {
		t.Errorf("lahan pertama diulang = %q, mau p1", got)
	}
	if got := table.Variety("aaaa"); got != "v1" {
		t.Errorf("varietas pertama = %q, mau v1", got)
	}
	if got := table.Commodity("bbbb"); got != "k1" {
		t.Errorf("komoditas pertama = %q, mau k1", got)
	}
}

func TestResolveReturnsTheOriginalIdentifier(t *testing.T) {
	table := NewRefTable()
	ref := table.Plot("11111111-1111-4111-8111-111111111111")

	id, ok := table.Resolve(ref)
	if !ok {
		t.Fatalf("Resolve(%q) tidak menemukan apa pun", ref)
	}
	if id != "11111111-1111-4111-8111-111111111111" {
		t.Errorf("Resolve(%q) = %q", ref, id)
	}

	if _, ok := table.Resolve("p99"); ok {
		t.Error("Resolve menerima ref yang tidak pernah diterbitkan")
	}
}

func TestRequestCarriesNoPersonalData(t *testing.T) {
	table := NewRefTable()

	request := Request{
		ContractVersion: "1.0",
		RequestID:       "5a1f0c9e-3c2a-4b3e-9f5a-77c0e2b1d004",
		Seed:            20260905,
		Season:          Season{Label: "MT I 2026/2027", Start: "2026-10-01", End: "2027-03-31"},
		Objectives:      []string{"aman", "pendapatan", "pasar"},
		Candidates: []Candidate{{
			ID:           "c001",
			PlotRef:      table.Plot("11111111-1111-4111-8111-111111111111"),
			AreaHa:       0.82,
			CommodityRef: table.Commodity("padi"),
			VarietyRef:   table.Variety("inpari-32"),
			PlantingDate: "2026-10-05",
			HarvestStart: "2027-01-02",
			HarvestEnd:   "2027-01-16",
			TonnesLow:    2.91,
			TonnesMid:    3.60,
			TonnesHigh:   4.42,
			Plausibility: "plausible",
		}},
	}

	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	payload := string(body)

	beracun := []string{
		"Bu Sri Wahyuni", "Sri", "Jalancagak", "Subang", "Jawa Barat",
		"-6.25", "107.75", "3213", "inpari-32", "padi",
		"11111111-1111-4111-8111-111111111111",
	}
	for _, jarum := range beracun {
		if strings.Contains(payload, jarum) {
			t.Errorf("payload memuat %q; kontrak tidak boleh membawa identitas "+
				"maupun pengenal stabil apa pun", jarum)
		}
	}
}

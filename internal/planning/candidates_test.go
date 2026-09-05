package planning_test

import (
	"fmt"
	"reflect"
	"testing"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/planning"
)

func sampleInput(t *testing.T) planning.CandidateInput {
	t.Helper()
	price := 5200.0

	return planning.CandidateInput{
		Season: planning.Season{
			Label: "MT I 2026/2027",
			Start: mustDate(t, "2026-10-01"),
			End:   mustDate(t, "2027-03-31"),
		},
		Plots: []planning.Plot{
			{ID: "plot-b", AreaHa: 1.35},
			{ID: "plot-a", AreaHa: 0.82},
		},
		Varieties: []planning.Variety{
			{ID: "var-2", CommodityID: "padi", Agronomy: agronomy.Variety{
				GddRequirement: 1500, BaseTempC: 10,
				DaysToHarvestMin: 90, DaysToHarvestMax: 115,
				YieldPerHaMin: 4, YieldPerHaMax: 7,
			}},
			{ID: "var-1", CommodityID: "padi", Agronomy: agronomy.Variety{
				GddRequirement: 1350, BaseTempC: 10,
				DaysToHarvestMin: 80, DaysToHarvestMax: 100,
				YieldPerHaMin: 3.5, YieldPerHaMax: 6,
			}},
		},
		Climatology: normalsForYear(27.8),
		PricePerKg:  map[string]*float64{"padi": &price},
	}
}

func TestBuildCandidatesCoversEveryPlotAndVariety(t *testing.T) {
	candidates := planning.BuildCandidates(sampleInput(t))

	if len(candidates) == 0 {
		t.Fatal("tidak ada kandidat yang dibangun")
	}

	plots := map[string]bool{}
	varieties := map[string]bool{}
	for _, candidate := range candidates {
		plots[candidate.PlotID] = true
		varieties[candidate.VarietyID] = true
	}

	if len(plots) != 2 || len(varieties) != 2 {
		t.Fatalf("harus mencakup 2 lahan dan 2 varietas, dapat %d dan %d", len(plots), len(varieties))
	}
}

func TestBuildCandidatesProducesOrderedTonnageRanges(t *testing.T) {
	for _, candidate := range planning.BuildCandidates(sampleInput(t)) {
		if !(candidate.TonnesLow <= candidate.TonnesMid && candidate.TonnesMid <= candidate.TonnesHigh) {
			t.Fatalf("%s tonase tidak terurut: %v %v %v",
				candidate.ID, candidate.TonnesLow, candidate.TonnesMid, candidate.TonnesHigh)
		}
		if candidate.TonnesLow < 0 {
			t.Fatalf("%s tonase negatif", candidate.ID)
		}
	}
}

func TestBuildCandidatesKeepsHarvestInsideTheSeason(t *testing.T) {
	input := sampleInput(t)

	for _, candidate := range planning.BuildCandidates(input) {
		if candidate.HarvestEnd.After(input.Season.End) {
			t.Fatalf("%s panen melewati akhir musim: %v", candidate.ID, candidate.HarvestEnd)
		}
		if candidate.PlantingDate.Before(input.Season.Start) {
			t.Fatalf("%s tanam sebelum musim dimulai: %v", candidate.ID, candidate.PlantingDate)
		}
		if !candidate.HarvestStart.After(candidate.PlantingDate) {
			t.Fatalf("%s panen tidak setelah tanam", candidate.ID)
		}
	}
}

func TestBuildCandidatesNeverOffersAnImplausibleWindow(t *testing.T) {
	for _, candidate := range planning.BuildCandidates(sampleInput(t)) {
		if candidate.Plausibility == constants.PlausibilityImplausible {
			t.Fatalf("%s ditawarkan padahal jendelanya tidak masuk akal", candidate.ID)
		}
	}
}

func TestBuildCandidatesIsDeterministic(t *testing.T) {
	first := planning.BuildCandidates(sampleInput(t))
	second := planning.BuildCandidates(sampleInput(t))

	if !reflect.DeepEqual(first, second) {
		t.Fatal("dua pembangunan dari masukan yang sama menghasilkan kandidat yang berbeda")
	}
}

func TestBuildCandidatesIgnoresTheOrderItReceivesInput(t *testing.T) {
	sorted := planning.BuildCandidates(sampleInput(t))

	shuffled := sampleInput(t)
	shuffled.Plots[0], shuffled.Plots[1] = shuffled.Plots[1], shuffled.Plots[0]
	shuffled.Varieties[0], shuffled.Varieties[1] = shuffled.Varieties[1], shuffled.Varieties[0]

	if !reflect.DeepEqual(sorted, planning.BuildCandidates(shuffled)) {
		t.Fatal("urutan masukan mengubah keluaran; determinisme bocor lewat urutan kueri")
	}
}

func TestBuildCandidatesRespectsTheSizeLimit(t *testing.T) {
	input := sampleInput(t)
	input.MaxCandidates = 5

	candidates := planning.BuildCandidates(input)

	if len(candidates) != 5 {
		t.Fatalf("batas 5 dilanggar, dapat %d", len(candidates))
	}
	if candidates[4].ID != "c005" {
		t.Fatalf("penomoran tidak berurutan: %s", candidates[4].ID)
	}
}

func TestBuildCandidatesSkipsPlotsWithoutArea(t *testing.T) {
	input := sampleInput(t)
	input.Plots = append(input.Plots, planning.Plot{ID: "plot-c", AreaHa: 0})

	for _, candidate := range planning.BuildCandidates(input) {
		if candidate.PlotID == "plot-c" {
			t.Fatal("lahan tanpa luas tidak boleh menghasilkan kandidat")
		}
	}
}

func TestBuildCandidatesCarriesThePriceOfItsCommodity(t *testing.T) {
	candidates := planning.BuildCandidates(sampleInput(t))

	if candidates[0].PricePerKg == nil || *candidates[0].PricePerKg != 5200 {
		t.Fatalf("harga acuan tidak terbawa: %+v", candidates[0].PricePerKg)
	}
}

func TestBuildCandidatesLeavesPriceNilForAnUnpricedCommodity(t *testing.T) {
	input := sampleInput(t)
	input.PricePerKg = nil

	if candidate := planning.BuildCandidates(input)[0]; candidate.PricePerKg != nil {
		t.Fatal("komoditas tanpa harga acuan harus kosong, bukan nol rupiah")
	}
}

func TestMergeCandidatesRenumbersWithoutGaps(t *testing.T) {
	first := planning.BuildCandidates(sampleInput(t))

	second := sampleInput(t)
	second.Plots = []planning.Plot{{ID: "plot-z", AreaHa: 0.5}}
	merged := planning.MergeCandidates(first, planning.BuildCandidates(second))

	if len(merged) != len(first)+len(planning.BuildCandidates(second)) {
		t.Fatalf("penggabungan kehilangan kandidat: %d", len(merged))
	}
	for i, candidate := range merged {
		want := fmt.Sprintf("c%03d", i+1)
		if candidate.ID != want {
			t.Fatalf("penomoran berlubang pada posisi %d: %s bukan %s", i, candidate.ID, want)
		}
	}
}

func TestMergeCandidatesIsDeterministic(t *testing.T) {
	build := func() []planning.Candidate {
		second := sampleInput(t)
		second.Plots = []planning.Plot{{ID: "plot-z", AreaHa: 0.5}}
		return planning.MergeCandidates(
			planning.BuildCandidates(sampleInput(t)), planning.BuildCandidates(second))
	}

	if !reflect.DeepEqual(build(), build()) {
		t.Fatal("penggabungan tidak deterministik")
	}
}

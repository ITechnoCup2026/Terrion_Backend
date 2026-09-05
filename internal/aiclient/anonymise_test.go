package aiclient_test

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/aiclient"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/planning"
)

var poisonous = []string{
	"Bu Sri Wahyuni",
	"Pak Ujang",
	"Jalancagak",
	"KUD Subang",
	"-6.2504",
	"107.7891",
	"3204012001010001",
	"11111111-1111-4111-8111-111111111111",
}

func mustDate(t *testing.T, iso string) time.Time {
	t.Helper()
	parsed, err := agronomy.UTCDate(iso)
	if err != nil {
		t.Fatalf("tanggal %q: %v", iso, err)
	}
	return parsed
}

func poisonedInput(t *testing.T) aiclient.RequestInput {
	t.Helper()
	price := 5200.0
	capacity := 12.5

	return aiclient.RequestInput{
		RequestID: "5a1f0c9e-3c2a-4b3e-9f5a-77c0e2b1d004",
		Seed:      20260905,
		Season: planning.Season{
			Label: "MT I 2026/2027",
			Start: mustDate(t, "2026-10-01"),
			End:   mustDate(t, "2027-03-31"),
		},
		Objectives: []planning.Objective{
			planning.ObjectiveSafe, planning.ObjectiveIncome, planning.ObjectiveMarket,
		},
		Capacity: &capacity,
		Candidates: []planning.Candidate{
			{
				ID: "c001", PlotID: "Bu Sri Wahyuni", AreaHa: 0.82,
				CommodityID: "Jalancagak", VarietyID: "-6.2504",
				PlantingDate: mustDate(t, "2026-10-05"),
				HarvestStart: mustDate(t, "2027-01-02"),
				HarvestEnd:   mustDate(t, "2027-01-16"),
				TonnesLow:    2.91, TonnesMid: 3.60, TonnesHigh: 4.42,
				Plausibility: constants.PlausibilityOk, PricePerKg: &price,
			},
			{
				ID: "c002", PlotID: "11111111-1111-4111-8111-111111111111", AreaHa: 1.35,
				CommodityID: "KUD Subang", VarietyID: "3204012001010001",
				PlantingDate: mustDate(t, "2026-10-12"),
				HarvestStart: mustDate(t, "2027-01-18"),
				HarvestEnd:   mustDate(t, "2027-02-01"),
				TonnesLow:    4.10, TonnesMid: 5.20, TonnesHigh: 6.30,
				Plausibility: constants.PlausibilityLate, PricePerKg: nil,
			},
		},
		Demand: []planning.DemandRow{
			{CommodityID: "Jalancagak", Week: mustDate(t, "2027-01-04"), Kg: 4000},
			{CommodityID: "Pak Ujang", Week: mustDate(t, "2027-01-11"), Kg: 1000},
		},
	}
}

func TestNoPersonalDataReachesTheWire(t *testing.T) {
	request, _ := aiclient.BuildRequest(poisonedInput(t))

	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	wire := string(payload)

	for _, secret := range poisonous {
		if strings.Contains(wire, secret) {
			t.Fatalf("payload memuat %q. Uji ini gagal pada hari seseorang menambah "+
				"field yang salah, bukan pada hari data bocor ke pihak ketiga.\n%s", secret, wire)
		}
	}
}

func TestEveryReferenceIsOpaque(t *testing.T) {
	request, _ := aiclient.BuildRequest(poisonedInput(t))
	pattern := regexp.MustCompile(`^[pkv][0-9]+$`)

	for _, candidate := range request.Candidates {
		for label, ref := range map[string]string{
			"plot_ref": candidate.PlotRef, "commodity_ref": candidate.CommodityRef,
			"variety_ref": candidate.VarietyRef,
		} {
			if !pattern.MatchString(ref) {
				t.Fatalf("%s %q bukan referensi buram", label, ref)
			}
		}
	}
	for _, row := range request.Demand {
		if !pattern.MatchString(row.CommodityRef) {
			t.Fatalf("commodity_ref permintaan %q bukan referensi buram", row.CommodityRef)
		}
	}
}

func TestReferencesAreStableWithinOneRequest(t *testing.T) {
	request, _ := aiclient.BuildRequest(poisonedInput(t))

	refByCommodity := map[string]string{}
	for _, candidate := range request.Candidates {
		refByCommodity[candidate.ID] = candidate.CommodityRef
	}
	if refByCommodity["c001"] == refByCommodity["c002"] {
		t.Fatal("dua komoditas berbeda memakai referensi yang sama")
	}
}

func TestBuildRequestIsDeterministic(t *testing.T) {
	first, _ := aiclient.BuildRequest(poisonedInput(t))
	second, _ := aiclient.BuildRequest(poisonedInput(t))

	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatal("dua pembangunan dari masukan yang sama menghasilkan payload berbeda")
	}
}

func TestBuildRequestIgnoresTheOrderCandidatesArriveIn(t *testing.T) {
	forward, _ := aiclient.BuildRequest(poisonedInput(t))

	shuffled := poisonedInput(t)
	shuffled.Candidates[0], shuffled.Candidates[1] = shuffled.Candidates[1], shuffled.Candidates[0]
	backward, _ := aiclient.BuildRequest(shuffled)

	a, _ := json.Marshal(forward)
	b, _ := json.Marshal(backward)
	if string(a) != string(b) {
		t.Fatal("urutan kandidat mengubah payload")
	}
}

func TestPlausibilityIsTranslatedToTheContractVocabulary(t *testing.T) {
	request, _ := aiclient.BuildRequest(poisonedInput(t))

	allowed := map[string]bool{"plausible": true, "early": true, "late": true}
	for _, candidate := range request.Candidates {
		if !allowed[candidate.Plausibility] {
			t.Fatalf("%s plausibility %q di luar kosakata kontrak", candidate.ID, candidate.Plausibility)
		}
	}
}

func TestImplausibleCandidatesAreNeverOffered(t *testing.T) {
	input := poisonedInput(t)
	input.Candidates[0].Plausibility = constants.PlausibilityImplausible

	request, mapping := aiclient.BuildRequest(input)

	if len(request.Candidates) != 1 || request.Candidates[0].ID != "c002" {
		t.Fatalf("kandidat tidak masuk akal ikut terkirim: %+v", request.Candidates)
	}
	if mapping.Knows("c001") {
		t.Fatal("pemetaan mengakui kandidat yang tidak pernah dikirim")
	}
}

func TestUnpricedCandidatesStayNullRatherThanZero(t *testing.T) {
	request, _ := aiclient.BuildRequest(poisonedInput(t))

	for _, candidate := range request.Candidates {
		if candidate.ID == "c002" && candidate.PricePerKg != nil {
			t.Fatal("komoditas tanpa harga acuan harus kosong, bukan nol rupiah")
		}
	}
}

func TestDemandWeeksAreMondays(t *testing.T) {
	request, _ := aiclient.BuildRequest(poisonedInput(t))

	for _, row := range request.Demand {
		if mustDate(t, row.ISOWeek).Weekday() != time.Monday {
			t.Fatalf("iso_week %q bukan hari Senin", row.ISOWeek)
		}
	}
}

func TestMappingRefusesCandidateItNeverIssued(t *testing.T) {
	_, mapping := aiclient.BuildRequest(poisonedInput(t))

	if mapping.Knows("c999") {
		t.Fatal("pemetaan menerima kandidat yang tidak pernah diterbitkan Go")
	}
	if !mapping.Knows("c001") {
		t.Fatal("pemetaan menolak kandidat yang ia terbitkan sendiri")
	}
}

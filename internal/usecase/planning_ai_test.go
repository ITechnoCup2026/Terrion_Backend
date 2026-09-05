package usecase

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"terrion-backend/internal/aiclient"
	"terrion-backend/internal/constants"
)

func balasanMemilihKandidatPertama(request aiclient.Request) aiclient.Response {
	terpilih := map[string]bool{}
	ids := []string{}
	for _, kandidat := range request.Candidates {
		if terpilih[kandidat.PlotRef] {
			continue
		}
		terpilih[kandidat.PlotRef] = true
		ids = append(ids, kandidat.ID)
	}

	plans := make([]aiclient.PlanResult, 0, len(request.Objectives))
	for _, objektif := range request.Objectives {
		plans = append(plans, aiclient.PlanResult{
			Objective:       objektif,
			CandidateIDs:    ids,
			Metrics:         aiclient.Metrics{PeakTonnesP50: 1, PeakTonnesP90: 2, TotalTonnes: 3},
			Narrative:       "narasi uji",
			NarrativeSource: "template",
		})
	}

	return aiclient.Response{
		ContractVersion: constants.AIContractVersion,
		RequestID:       request.RequestID,
		Solver:          "greedy",
		SolverVersion:   "uji",
		Plans:           plans,
		Diagnostics:     aiclient.Diagnostics{ObjectiveStatus: "OPTIMAL"},
	}
}

func TestProposeUsesTheAIServiceWhenItAnswers(t *testing.T) {
	db := seedPlanningFixture(t)

	var terlihat aiclient.Request

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&terlihat)
		_ = json.NewEncoder(w).Encode(balasanMemilihKandidatPertama(terlihat))
	}))
	defer server.Close()

	useCase := planningUseCase(t, db)
	useCase.AI = aiclient.NewClient(server.URL, "token-uji", 2*time.Second)

	proposal, err := useCase.Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	if proposal.Engine != constants.PlanEngineAIService {
		t.Errorf("Engine = %q, mau %q", proposal.Engine, constants.PlanEngineAIService)
	}
	if len(terlihat.Candidates) == 0 {
		t.Fatal("layanan AI tidak menerima satu pun kandidat")
	}
	if terlihat.ContractVersion != constants.AIContractVersion {
		t.Errorf("contract_version terkirim = %q", terlihat.ContractVersion)
	}
	if len(proposal.Plans) != 3 {
		t.Errorf("len(Plans) = %d, mau 3", len(proposal.Plans))
	}
	if proposal.Plans[0].Narrative != "narasi uji" {
		t.Errorf("Narrative = %q, mau diteruskan apa adanya", proposal.Plans[0].Narrative)
	}
}

func TestProposeFallsBackWhenTheAIServiceIsDown(t *testing.T) {
	db := seedPlanningFixture(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	server.Close()

	useCase := planningUseCase(t, db)
	useCase.AI = aiclient.NewClient(server.URL, "token-uji", 200*time.Millisecond)

	proposal, err := useCase.Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("Propose harus tetap berhasil saat layanan AI mati: %v", err)
	}
	if proposal.Engine != constants.PlanEngineFallback {
		t.Errorf("Engine = %q, mau %q", proposal.Engine, constants.PlanEngineFallback)
	}
	if len(proposal.Plans) != 3 {
		t.Errorf("len(Plans) = %d, mau 3 walaupun memakai solver lokal", len(proposal.Plans))
	}
	for _, rencana := range proposal.Plans {
		if len(rencana.Assignments) == 0 {
			t.Errorf("rencana %q kosong pada jalur fallback", rencana.Objective)
		}
	}
}

func TestProposeWithoutAnAIServiceBehavesExactlyAsBefore(t *testing.T) {
	db := seedPlanningFixture(t)

	useCase := planningUseCase(t, db)
	useCase.AI = nil

	proposal, err := useCase.Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if proposal.Engine != constants.PlanEngineFallback {
		t.Errorf("Engine = %q", proposal.Engine)
	}
}

func TestProposeIgnoresCandidateIDsTheAIServiceInvented(t *testing.T) {
	db := seedPlanningFixture(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request aiclient.Request
		_ = json.NewDecoder(r.Body).Decode(&request)

		balasan := balasanMemilihKandidatPertama(request)
		balasan.Plans[0].CandidateIDs = append(balasan.Plans[0].CandidateIDs, "c9999")
		_ = json.NewEncoder(w).Encode(balasan)
	}))
	defer server.Close()

	useCase := planningUseCase(t, db)
	useCase.AI = aiclient.NewClient(server.URL, "token-uji", 2*time.Second)

	proposal, err := useCase.Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	for _, penugasan := range proposal.Plans[0].Assignments {
		if penugasan.PlotID == "" {
			t.Fatal("kandidat karangan lolos ke rencana; setiap penugasan wajib berasal " +
				"dari tabel kandidat yang backend sendiri terbitkan")
		}
	}
}

func TestProposeNeverTrustsMetricsFromTheAIService(t *testing.T) {
	db := seedPlanningFixture(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request aiclient.Request
		_ = json.NewDecoder(r.Body).Decode(&request)

		balasan := balasanMemilihKandidatPertama(request)
		for i := range balasan.Plans {
			balasan.Plans[i].Metrics.TotalTonnes = 999999
			balasan.Plans[i].Metrics.PeakTonnesP50 = 999999
		}
		_ = json.NewEncoder(w).Encode(balasan)
	}))
	defer server.Close()

	useCase := planningUseCase(t, db)
	useCase.AI = aiclient.NewClient(server.URL, "token-uji", 2*time.Second)

	proposal, err := useCase.Propose(context.Background(), homeCoop, planSeason, planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	for _, rencana := range proposal.Plans {
		if rencana.Metrics.TotalTonnesMid > 100000 {
			t.Errorf("rencana %q memakai tonase dari layanan AI (%.0f) alih-alih "+
				"menghitungnya ulang", rencana.Objective, rencana.Metrics.TotalTonnesMid)
		}
	}
}

func TestProposeSendsNoPersonalDataToTheAIService(t *testing.T) {
	db := seedPlanningFixture(t)

	var mentah []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mentah, _ = io.ReadAll(r.Body)

		var request aiclient.Request
		_ = json.Unmarshal(mentah, &request)
		_ = json.NewEncoder(w).Encode(balasanMemilihKandidatPertama(request))
	}))
	defer server.Close()

	useCase := planningUseCase(t, db)
	useCase.AI = aiclient.NewClient(server.URL, "token-uji", 2*time.Second)

	if _, err := useCase.Propose(
		context.Background(), homeCoop, planSeason, planningNow); err != nil {
		t.Fatalf("Propose: %v", err)
	}

	payload := string(mentah)
	beracun := []string{
		"Anggota plot-1", "KUD Uji", "Desa", "Kabupaten", "Jawa Barat", "Ciherang",
		"plot-1", "member-plot-1", "variety-rice", riceCommodity, homeCoop, "-6.25",
	}
	for _, jarum := range beracun {
		if strings.Contains(payload, jarum) {
			t.Errorf("muatan ke layanan AI memuat %q", jarum)
		}
	}
}

package aiclient_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"terrion-backend/internal/aiclient"
)

func TestAgainstARunningAiService(t *testing.T) {
	baseURL := os.Getenv("TERRION_AI_URL")
	if baseURL == "" {
		t.Skip("setel TERRION_AI_URL dan TERRION_AI_TOKEN untuk menguji terhadap layanan yang berjalan")
	}

	var request aiclient.Request
	if err := json.Unmarshal(readGolden(t, "propose_request.golden.json"), &request); err != nil {
		t.Fatalf("urai berkas emas: %v", err)
	}

	client := aiclient.New(baseURL, os.Getenv("TERRION_AI_TOKEN"), 5*time.Second, nil)

	response, err := client.Propose(context.Background(), request)
	if err != nil {
		t.Fatalf("layanan menolak berkas emas: %v", err)
	}

	if response.ContractVersion != aiclient.ContractVersion {
		t.Fatalf("layanan berbicara kontrak %q", response.ContractVersion)
	}
	if len(response.Plans) != len(request.Objectives) {
		t.Fatalf("%d objektif diminta, %d rencana kembali", len(request.Objectives), len(response.Plans))
	}

	byObjective := map[string]aiclient.PlanResult{}
	for _, plan := range response.Plans {
		byObjective[plan.Objective] = plan
		if plan.Metrics.PeakTonnesP90 < plan.Metrics.PeakTonnesP50 {
			t.Fatalf("%s: P90 di bawah P50", plan.Objective)
		}
		if plan.Narrative == nil || *plan.Narrative == "" {
			t.Fatalf("%s: tidak ada narasi", plan.Objective)
		}
	}

	safe := byObjective["aman"].Metrics.PeakTonnesP90
	if safe > byObjective["pendapatan"].Metrics.PeakTonnesP90 ||
		safe > byObjective["pasar"].Metrics.PeakTonnesP90 {
		t.Fatal("rencana aman tidak punya puncak P90 terendah")
	}
}

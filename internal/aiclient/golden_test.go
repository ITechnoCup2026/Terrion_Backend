package aiclient_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"terrion-backend/internal/aiclient"
)

func readGolden(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("baca %s: %v", name, err)
	}
	return raw
}

func asMap(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("urai: %v", err)
	}
	return out
}

func TestGoldenRequestSurvivesTheGoTypesUnchanged(t *testing.T) {
	raw := readGolden(t, "propose_request.golden.json")

	var request aiclient.Request
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatalf("berkas emas tidak terurai ke tipe Go: %v", err)
	}

	roundTripped, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if !reflect.DeepEqual(asMap(t, raw), asMap(t, roundTripped)) {
		t.Fatal("bentuk permintaan di sisi Go menyimpang dari berkas emas. " +
			"Kalau perubahan ini disengaja, perbarui berkas emas di KEDUA repo " +
			"dan naikkan contract_version.")
	}
}

func TestGoldenResponseSurvivesTheGoTypesUnchanged(t *testing.T) {
	raw := readGolden(t, "propose_response.golden.json")

	var response aiclient.Response
	if err := json.Unmarshal(raw, &response); err != nil {
		t.Fatalf("berkas emas tidak terurai ke tipe Go: %v", err)
	}

	roundTripped, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if !reflect.DeepEqual(asMap(t, raw), asMap(t, roundTripped)) {
		t.Fatal("bentuk respons di sisi Go menyimpang dari berkas emas")
	}
}

func TestGoldenRequestCarriesTheContractVersionWeSpeak(t *testing.T) {
	var request aiclient.Request
	if err := json.Unmarshal(readGolden(t, "propose_request.golden.json"), &request); err != nil {
		t.Fatalf("urai: %v", err)
	}

	if request.ContractVersion != aiclient.ContractVersion {
		t.Fatalf("berkas emas memakai kontrak %q, layanan ini berbicara %q",
			request.ContractVersion, aiclient.ContractVersion)
	}
}

func TestGoldenResponseHasOnePlanPerRequestedObjective(t *testing.T) {
	var request aiclient.Request
	var response aiclient.Response
	if err := json.Unmarshal(readGolden(t, "propose_request.golden.json"), &request); err != nil {
		t.Fatalf("urai permintaan: %v", err)
	}
	if err := json.Unmarshal(readGolden(t, "propose_response.golden.json"), &response); err != nil {
		t.Fatalf("urai respons: %v", err)
	}

	if len(response.Plans) != len(request.Objectives) {
		t.Fatalf("%d objektif diminta, %d rencana kembali",
			len(request.Objectives), len(response.Plans))
	}
	for i, objective := range request.Objectives {
		if response.Plans[i].Objective != objective {
			t.Fatalf("urutan rencana tidak mengikuti urutan objektif pada posisi %d", i)
		}
	}
}

func TestGoldenRequestNeverCarriesAUuid(t *testing.T) {
	raw := string(readGolden(t, "propose_request.golden.json"))
	var request aiclient.Request
	if err := json.Unmarshal([]byte(raw), &request); err != nil {
		t.Fatalf("urai: %v", err)
	}

	for _, candidate := range request.Candidates {
		for _, ref := range []string{candidate.PlotRef, candidate.CommodityRef, candidate.VarietyRef} {
			if len(ref) > 8 {
				t.Fatalf("referensi %q terlalu panjang untuk sebuah referensi buram", ref)
			}
		}
	}
}

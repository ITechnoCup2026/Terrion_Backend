package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"terrion-backend/internal/aiclient"
	"terrion-backend/internal/constants"
)

// tujuanYangTerkirim menjalankan Propose dan mengembalikan badan permintaan
// mentah yang diterima layanan AI. Yang diperiksa berkas ini adalah apa yang
// benar-benar melintas kabel, bukan struct yang kebetulan terisi di memori.
func tujuanYangTerkirim(t *testing.T, goal string) (map[string]any, error) {
	t.Helper()

	db := seedPlanningFixture(t)

	var badan map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mentah, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(mentah, &badan)

		var request aiclient.Request
		_ = json.Unmarshal(mentah, &request)
		_ = json.NewEncoder(w).Encode(balasanMemilihKandidatPertama(request))
	}))
	defer server.Close()

	useCase := planningUseCase(t, db)
	useCase.AI = aiclient.NewClient(server.URL, "token-uji", 2*time.Second)

	_, err := useCase.Propose(context.Background(), homeCoop, planSeason, goal, planningNow)
	return badan, err
}

func TestTheGoalReachesTheAIService(t *testing.T) {
	tujuan := "Musim depan jangan sampai harga jatuh seperti kemarin"

	badan, err := tujuanYangTerkirim(t, tujuan)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	if badan["goal"] != tujuan {
		t.Errorf("goal terkirim = %#v, mau %q — tanpa ini lapis tujuan di layanan "+
			"AI tidak pernah bisa dijangkau dari backend", badan["goal"], tujuan)
	}
}

func TestASurroundingBlankGoalIsTreatedAsNoGoalAtAll(t *testing.T) {
	// Kolom kosong yang dikirim sebagai spasi tidak boleh membelanjakan satu
	// panggilan model pun: anggaran waktu itu milik narasi.
	for _, goal := range []string{"", "   ", "\t\n"} {
		badan, err := tujuanYangTerkirim(t, goal)
		if err != nil {
			t.Fatalf("Propose(%q): %v", goal, err)
		}

		if _, ada := badan["goal"]; ada {
			t.Errorf("permintaan tanpa tujuan tetap membawa kunci goal (%q); "+
				"sidik jari cache lama ikut berubah tanpa alasan", goal)
		}
	}
}

func TestAGoalLongerThanTheContractIsRefusedInsteadOfLosingThePlan(t *testing.T) {
	// Layanan AI menolak tujuan di atas 500 aksara mentah-mentah. Kalau backend
	// meneruskannya begitu saja, satu kalimat kepanjangan membuat pengurus
	// kehilangan seluruh rencananya, bukan sekadar tujuannya.
	badan, err := tujuanYangTerkirim(t, strings.Repeat("a", constants.PlanGoalMaxChars+1))

	var refusal *PlanRefusal
	if !errors.As(err, &refusal) || refusal.Code != constants.PlanGoalTooLong {
		t.Fatalf("err = %v, mau penolakan %q", err, constants.PlanGoalTooLong)
	}
	if badan != nil {
		t.Error("tujuan kepanjangan tetap dikirim ke layanan AI")
	}
}

func TestAGoalExactlyAtTheLimitIsStillAccepted(t *testing.T) {
	tujuan := strings.Repeat("a", constants.PlanGoalMaxChars)

	badan, err := tujuanYangTerkirim(t, tujuan)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if badan["goal"] != tujuan {
		t.Error("tujuan tepat sepanjang batas ikut ditolak")
	}
}

func TestTheLimitCountsLettersNotBytes(t *testing.T) {
	// Batas di sisi Python dihitung dalam aksara. Menghitung byte di sini
	// membuat tujuan beraksara non-ASCII ditolak jauh sebelum batasnya.
	tujuan := strings.Repeat("é", constants.PlanGoalMaxChars)

	if _, err := tujuanYangTerkirim(t, tujuan); err != nil {
		t.Fatalf("tujuan %d aksara (%d byte) ditolak: %v",
			constants.PlanGoalMaxChars, len(tujuan), err)
	}
}

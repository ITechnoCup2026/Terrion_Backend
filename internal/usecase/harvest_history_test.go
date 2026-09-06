package usecase

import (
	"context"
	"testing"

	"terrion-backend/internal/entity"
)

// Panen yang tercatat tetap bisa dibaca.
//
// Mencatat panen menutup bloknya: ia hilang dari kanvas lahan, karena tidak ada
// lagi yang tumbuh di sana. Tetapi hilang dari layar dan hilang dari catatan
// adalah dua hal berbeda, dan sebelum ini keduanya sama -- seorang kader yang
// mencatat panen kehilangan satu-satunya bukti bahwa ia pernah mencatatnya.
//
// Angka-angka ini juga yang melatih kalibrasi koperasi. Model yang belajar dari
// catatan yang tidak bisa diperiksa siapa pun adalah model yang tidak bisa
// dibantah.
func TestHarvestHistoryListsRecordedHarvestsNewestFirst(t *testing.T) {
	db, user := plotFixture(t)

	rows, err := plotUseCase(t, db).HarvestHistory(context.Background(), user)
	if err != nil {
		t.Fatalf("HarvestHistory: %v", err)
	}

	if len(rows) == 0 {
		t.Fatal("len(rows) = 0 — fixture punya satu blok yang sudah dipanen")
	}

	for _, row := range rows {
		if row.PlotName == "" {
			t.Errorf("baris %s tidak menyebut nama lahan", row.BlockID)
		}
		if row.CommodityName == "" || row.VarietyName == "" {
			t.Errorf("baris %s tidak menyebut komoditas/varietas", row.BlockID)
		}
		if row.ActualYieldKg <= 0 {
			t.Errorf("baris %s punya hasil %v, want lebih dari 0", row.BlockID, row.ActualYieldKg)
		}
	}

	for i := 1; i < len(rows); i++ {
		if rows[i-1].HarvestDate.Before(rows[i].HarvestDate) {
			t.Errorf("baris %d lebih tua dari baris %d — urutannya harus terbaru dulu", i-1, i)
		}
	}
}

// Riwayat satu koperasi berhenti di batas koperasi itu.
func TestHarvestHistoryIsScopedToTheCooperative(t *testing.T) {
	db, user := plotFixture(t)

	rows, err := plotUseCase(t, db).HarvestHistory(context.Background(), user)
	if err != nil {
		t.Fatalf("HarvestHistory: %v", err)
	}

	for _, row := range rows {
		if row.BlockID == "block-other-coop" {
			t.Errorf("riwayat memuat blok koperasi lain: %s", row.BlockID)
		}
	}
}

func TestHarvestHistoryRefusesAnAccountWithoutACooperative(t *testing.T) {
	db, _ := plotFixture(t)

	_, err := plotUseCase(t, db).HarvestHistory(
		context.Background(), &entity.AppUser{ID: "buyer-1"})

	if err != ErrNoCooperative {
		t.Errorf("err = %v, want ErrNoCooperative", err)
	}
}

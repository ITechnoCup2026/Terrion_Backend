package usecase

import (
	"context"
	"io"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/repository"
)

func capacityUseCase(t *testing.T, db *gorm.DB) *CapacityUseCase {
	t.Helper()
	log := logrus.New()
	log.SetOutput(io.Discard)

	return NewCapacityUseCase(db, log, validator.New(),
		&repository.CooperativeRepository{}, &repository.CommodityRepository{})
}

// Setiap komoditas acuan muncul, termasuk yang belum punya angka.
//
// Kapasitas yang belum diisi adalah null, bukan nol: "gudang ini tidak bisa
// menampung apa pun" dan "belum ada yang mengukur gudang ini" adalah dua
// pernyataan berbeda, dan yang kedua yang benar sebelum pengurus mengisinya.
// Ambang deteksi tabrakan membedakan keduanya -- tanpa kapasitas ia memakai
// median, bukan nol.
func TestCapacityLoadListsEveryCommodityIncludingUnset(t *testing.T) {
	db := dashboardFixture(t)

	loaded, err := capacityUseCase(t, db).Load(context.Background(), pengurus())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded) == 0 {
		t.Fatal("len(rows) = 0, want one row per komoditas acuan")
	}

	unset := 0
	for _, row := range loaded {
		if row.CommodityName == "" {
			t.Errorf("baris %s tidak punya nama komoditas", row.CommodityID)
		}
		if row.TonnesPerWeek == nil {
			unset++
		}
	}
	if unset == 0 {
		t.Error("tidak ada baris yang null; fixture ini belum mengisi kapasitas apa pun")
	}
}

func TestCapacitySaveStoresAndReadsBack(t *testing.T) {
	db := dashboardFixture(t)
	use := capacityUseCase(t, db)

	tonnes := 18.5
	err := use.Save(context.Background(), pengurus(), &model.SetCapacityRequest{
		Rows: []model.SetCapacityRow{
			{CommodityID: maizeCommodity, TonnesPerWeek: &tonnes},
		},
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := use.Load(context.Background(), pengurus())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, row := range loaded {
		if row.CommodityID != maizeCommodity {
			continue
		}
		if row.TonnesPerWeek == nil || *row.TonnesPerWeek != 18.5 {
			t.Errorf("TonnesPerWeek = %v, want 18.5", row.TonnesPerWeek)
		}
		return
	}
	t.Errorf("komoditas %s tidak ada di hasil Load", maizeCommodity)
}

// Mengosongkan satu baris menghapusnya, bukan menyimpan nol.
//
// Tabelnya sendiri menolak nol (check tonnes_per_week > 0), jadi menyimpannya
// akan gagal di basis data. Tetapi alasannya bukan kendala teknis: pengurus
// yang menghapus angkanya sedang berkata "saya tidak tahu", dan deteksi
// tabrakan harus kembali memakai median seperti sebelum angka itu ada.
func TestCapacitySaveClearsARowWhenTonnesAreOmitted(t *testing.T) {
	db := dashboardFixture(t)
	use := capacityUseCase(t, db)

	tonnes := 12.0
	if err := use.Save(context.Background(), pengurus(), &model.SetCapacityRequest{
		Rows: []model.SetCapacityRow{{CommodityID: maizeCommodity, TonnesPerWeek: &tonnes}},
	}); err != nil {
		t.Fatalf("Save pertama: %v", err)
	}

	if err := use.Save(context.Background(), pengurus(), &model.SetCapacityRequest{
		Rows: []model.SetCapacityRow{{CommodityID: maizeCommodity, TonnesPerWeek: nil}},
	}); err != nil {
		t.Fatalf("Save kedua: %v", err)
	}

	rows := []entity.CooperativeCapacity{}
	if err := db.Where("cooperative_id = ? AND commodity_id = ?",
		homeCoop, maizeCommodity).Find(&rows).Error; err != nil {
		t.Fatalf("membaca ulang: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("masih ada %d baris, want 0 — kapasitas kosong berarti barisnya hilang", len(rows))
	}
}

func TestCapacityRefusesAnAccountWithoutACooperative(t *testing.T) {
	db := dashboardFixture(t)

	_, err := capacityUseCase(t, db).Load(context.Background(), &entity.AppUser{ID: "buyer-1"})

	if err != ErrNoCooperative {
		t.Errorf("err = %v, want ErrNoCooperative", err)
	}
}

package usecase

import (
	"context"
	"errors"
	"testing"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/plots"
)

// Luas berubah, dan hanya luas yang berubah.
//
// Lahan menyusut dan meluas: sepetak dijual, sepetak disewa, batasnya diukur
// ulang. Sebelum ini satu-satunya cara memperbaikinya adalah mendaftarkan lahan
// baru dan meninggalkan yang lama, yang berarti proyeksi koperasi menghitung
// hektare yang sama dua kali.
func TestUpdateBlockChangesTheArea(t *testing.T) {
	db, user := plotFixture(t)

	err := plotUseCase(t, db).UpdateBlock(context.Background(), user, "block-growing",
		&model.UpdateBlockRequest{AreaHa: 2.5})
	if err != nil {
		t.Fatalf("UpdateBlock: %v", err)
	}

	stored := new(entity.Block)
	if err := db.Where("id = ?", "block-growing").Take(stored).Error; err != nil {
		t.Fatalf("membaca ulang: %v", err)
	}
	if stored.AreaHa != 2.5 {
		t.Errorf("AreaHa = %v, want 2.5", stored.AreaHa)
	}
}

// Blok yang sudah dipanen tidak bisa disunting.
//
// Panen yang tercatat sudah masuk ke kalibrasi model hasil koperasi ini.
// Mengubah luas atau varietasnya setelah itu berarti membuat catatan panen
// menggambarkan sesuatu yang tidak pernah ditanam, dan kalibrasi yang lahir
// darinya tidak lagi bisa dipertanggungjawabkan.
func TestUpdateBlockRefusesAHarvestedBlock(t *testing.T) {
	db, user := plotFixture(t)

	err := plotUseCase(t, db).UpdateBlock(context.Background(), user, "block-harvested",
		&model.UpdateBlockRequest{AreaHa: 2.5})

	var refusal *plots.EditRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v, want *plots.EditRefusal", err)
	}
	if refusal.Code != constants.EditBlockHarvested {
		t.Errorf("Code = %q, want %q", refusal.Code, constants.EditBlockHarvested)
	}
}

func TestUpdateBlockRefusesABlockOfAnotherCooperative(t *testing.T) {
	db, user := plotFixture(t)

	// Blok milik koperasi lain, dilihat dari koperasi ini.
	err := plotUseCase(t, db).UpdateBlock(context.Background(), user, "block-other-coop",
		&model.UpdateBlockRequest{AreaHa: 2.5})

	var refusal *plots.EditRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v, want *plots.EditRefusal", err)
	}
	if refusal.Code != constants.EditBlockAlreadyGone {
		t.Errorf("Code = %q, want %q", refusal.Code, constants.EditBlockAlreadyGone)
	}
}

// Menghapus lahan menghapus blok-bloknya, dan hanya itu.
func TestDeletePlotRemovesItAndItsBlocks(t *testing.T) {
	db, user := plotFixture(t)

	// plot-home membawa panen tercatat, jadi tidak bisa dipakai di sini.
	// Lahan bersih dibuat khusus, seperti lahan yang baru saja salah didaftarkan.
	seedPlot(t, db, "plot-fresh", homeCoop, homeCell)
	if err := db.Create(&entity.Block{
		ID: "block-fresh", PlotID: "plot-fresh", Label: "BLOK A", AreaHa: 1,
		CommodityID: maizeCommodity, VarietyID: "variety-1",
		PlantingDate: agronomy.AddDays(projectionNow, -10),
	}).Error; err != nil {
		t.Fatalf("seeding the fresh plot: %v", err)
	}

	if err := plotUseCase(t, db).DeletePlot(
		context.Background(), user, "plot-fresh"); err != nil {
		t.Fatalf("DeletePlot: %v", err)
	}

	var plotCount, blockCount int64
	db.Model(&entity.Plot{}).Where("id = ?", "plot-fresh").Count(&plotCount)
	db.Model(&entity.Block{}).Where("plot_id = ?", "plot-fresh").Count(&blockCount)

	if plotCount != 0 {
		t.Errorf("lahan masih ada (%d baris)", plotCount)
	}
	if blockCount != 0 {
		t.Errorf("blok lahan itu masih ada (%d baris)", blockCount)
	}
}

// Lahan yang punya panen tercatat tidak boleh dihapus.
//
// Panen adalah catatan bahwa sesuatu benar-benar terjadi, dan kalibrasi model
// koperasi ini berdiri di atasnya. Menghapusnya diam-diam mengubah setiap
// proyeksi berikutnya tanpa ada yang bisa menjelaskan kenapa.
func TestDeletePlotRefusesWhenAHarvestIsRecorded(t *testing.T) {
	db, user := plotFixture(t)

	// plot-home punya BLOK B yang sudah dipanen.
	err := plotUseCase(t, db).DeletePlot(context.Background(), user, "plot-home")

	var refusal *plots.EditRefusal
	if !errors.As(err, &refusal) {
		t.Fatalf("err = %v, want *plots.EditRefusal", err)
	}
	if refusal.Code != constants.DeletePlotHarvested {
		t.Errorf("Code = %q, want %q", refusal.Code, constants.DeletePlotHarvested)
	}
}

package usecase

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/repository"
)

// CapacityUseCase memelihara berapa ton per minggu yang sanggup diserap
// koperasi per komoditas.
//
// Angka ini adalah ambang yang dipakai deteksi tabrakan. Sebelum ada layar
// untuk mengisinya, satu-satunya cara mengubahnya adalah lewat seeder atau SQL
// langsung -- artinya setiap koperasi selain yang di-seed berjalan dengan
// ambang median, dan tidak ada yang bisa memperbaikinya sendiri.
type CapacityUseCase struct {
	DB                    *gorm.DB
	Log                   *logrus.Logger
	Validate              *validator.Validate
	CooperativeRepository *repository.CooperativeRepository
	CommodityRepository   *repository.CommodityRepository
}

func NewCapacityUseCase(
	db *gorm.DB, log *logrus.Logger, validate *validator.Validate,
	cooperativeRepository *repository.CooperativeRepository,
	commodityRepository *repository.CommodityRepository,
) *CapacityUseCase {
	return &CapacityUseCase{
		DB:                    db,
		Log:                   log,
		Validate:              validate,
		CooperativeRepository: cooperativeRepository,
		CommodityRepository:   commodityRepository,
	}
}

// Load menyusun satu baris per komoditas acuan, terisi atau tidak.
//
// Setiap komoditas ikut disebut supaya layarnya adalah daftar yang bisa
// dilengkapi, bukan daftar pendek berisi apa yang kebetulan sudah ada. Yang
// belum diukur bernilai null.
func (u *CapacityUseCase) Load(
	ctx context.Context, user *entity.AppUser,
) ([]model.CapacityRowResponse, error) {
	if user.CooperativeID == nil {
		return nil, ErrNoCooperative
	}

	db := u.DB.WithContext(ctx)

	commodities, err := u.CommodityRepository.FindAll(db)
	if err != nil {
		return nil, fmt.Errorf("reading the commodities: %w", err)
	}

	stored, err := u.CooperativeRepository.FindCapacity(db, *user.CooperativeID)
	if err != nil {
		return nil, fmt.Errorf("reading the capacity: %w", err)
	}

	tonnes := make(map[string]float64, len(stored))
	for _, row := range stored {
		tonnes[row.CommodityID] = row.TonnesPerWeek
	}

	rows := make([]model.CapacityRowResponse, 0, len(commodities))
	for _, commodity := range commodities {
		row := model.CapacityRowResponse{
			CommodityID:   commodity.ID,
			CommodityName: commodity.Name,
		}
		if value, measured := tonnes[commodity.ID]; measured {
			// Disalin, karena mengambil alamat variabel perulangan akan
			// membuat setiap baris menunjuk angka yang sama.
			measured := value
			row.TonnesPerWeek = &measured
		}
		rows = append(rows, row)
	}

	return rows, nil
}

// Save menuliskan tabel kapasitas yang dikirim formulir.
//
// Baris tanpa angka dihapus, bukan disimpan sebagai nol: pengurus yang
// mengosongkan kolom sedang berkata "saya tidak tahu", dan ambangnya harus
// kembali ke median seperti sebelum angka itu pernah ada.
func (u *CapacityUseCase) Save(
	ctx context.Context, user *entity.AppUser, request *model.SetCapacityRequest,
) error {
	if user.CooperativeID == nil {
		return ErrNoCooperative
	}
	if err := u.Validate.Struct(request); err != nil {
		return err
	}

	db := u.DB.WithContext(ctx)

	// Komoditas yang tidak dikenal ditolak seluruhnya, bukan diabaikan diam-
	// diam: sebuah id yang salah ketik yang tersimpan tanpa keluhan adalah
	// kapasitas yang pengurus yakin sudah ia atur, dan yang tidak pernah
	// dipakai ambang mana pun.
	known, err := u.CommodityRepository.FindAll(db)
	if err != nil {
		return fmt.Errorf("reading the commodities: %w", err)
	}
	exists := make(map[string]bool, len(known))
	for _, commodity := range known {
		exists[commodity.ID] = true
	}

	write := []entity.CooperativeCapacity{}
	clear := []string{}

	for _, row := range request.Rows {
		if !exists[row.CommodityID] {
			return fmt.Errorf("%w: %s", ErrCommodityUnknown, row.CommodityID)
		}
		if row.TonnesPerWeek == nil {
			clear = append(clear, row.CommodityID)
			continue
		}
		write = append(write, entity.CooperativeCapacity{
			CooperativeID: *user.CooperativeID,
			CommodityID:   row.CommodityID,
			TonnesPerWeek: *row.TonnesPerWeek,
		})
	}

	if err := u.CooperativeRepository.ReplaceCapacity(
		db, *user.CooperativeID, write, clear); err != nil {
		return fmt.Errorf("writing the capacity: %w", err)
	}

	return nil
}

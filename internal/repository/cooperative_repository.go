package repository

import (
	"errors"

	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

type CooperativeRepository struct {
	Repository[entity.Cooperative]
}

func (r *CooperativeRepository) FindCapacity(
	db *gorm.DB, cooperativeID string,
) ([]entity.CooperativeCapacity, error) {
	capacity := []entity.CooperativeCapacity{}
	err := db.Where("cooperative_id = ?", cooperativeID).Find(&capacity).Error
	return capacity, err
}

// ReplaceCapacity menuliskan kapasitas untuk komoditas yang disebut, dan
// menghapus yang nilainya kosong.
//
// Hanya komoditas yang ada di `rows` yang disentuh: formulir boleh mengirim
// sebagian tabel tanpa diam-diam menghapus sisanya. Seluruhnya berjalan dalam
// satu transaksi, karena separuh tabel kapasitas yang tersimpan adalah ambang
// tabrakan yang tidak pernah dimaksudkan siapa pun.
func (r *CooperativeRepository) ReplaceCapacity(
	db *gorm.DB, cooperativeID string, rows []entity.CooperativeCapacity, clear []string,
) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if len(clear) > 0 {
			if err := tx.Where("cooperative_id = ? AND commodity_id IN ?",
				cooperativeID, clear).
				Delete(&entity.CooperativeCapacity{}).Error; err != nil {
				return err
			}
		}

		for _, row := range rows {
			// Upsert: satu komoditas punya paling banyak satu kapasitas, dan
			// kunci utamanya sudah menyatakan itu.
			if err := tx.Save(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *CooperativeRepository) FindAll(db *gorm.DB) ([]entity.Cooperative, error) {
	cooperatives := []entity.Cooperative{}
	err := db.Order("name").Find(&cooperatives).Error
	return cooperatives, err
}

func (r *CooperativeRepository) FindInVillage(
	db *gorm.DB, village, district string,
) (*entity.Cooperative, error) {
	cooperative := new(entity.Cooperative)
	err := db.Where("village = ? AND district = ?", village, district).
		Order("name").Take(cooperative).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return cooperative, nil
}

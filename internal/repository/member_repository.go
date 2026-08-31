package repository

import (
	"errors"

	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

type MemberRepository struct {
	Repository[entity.Member]
}

func (r *MemberRepository) FindByNameInCooperative(
	db *gorm.DB, cooperativeID, name string,
) (*entity.Member, error) {
	member := new(entity.Member)
	err := db.Where("cooperative_id = ? AND LOWER(name) = LOWER(?)", cooperativeID, name).
		Take(member).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return member, nil
}

func (r *MemberRepository) FindByCooperativeID(
	db *gorm.DB, cooperativeID string,
) ([]entity.Member, error) {
	members := []entity.Member{}
	err := db.Where("cooperative_id = ?", cooperativeID).Order("name").Find(&members).Error
	return members, err
}

package entity

import (
	"encoding/json"
	"time"

	"terrion-backend/internal/constants"
)

type Cooperative struct {
	ID             string          `gorm:"column:id;primaryKey"`
	Name           string          `gorm:"column:name"`
	Village        string          `gorm:"column:village"`
	District       string          `gorm:"column:district"`
	DistrictCode   *string         `gorm:"column:district_code"`
	Province       string          `gorm:"column:province"`
	Lat            float64         `gorm:"column:lat"`
	Lng            float64         `gorm:"column:lng"`
	StaggerApplied json.RawMessage `gorm:"column:stagger_applied;type:jsonb"`
	// Phone adalah kontak koperasi, bukan kontak pengurus: kepengurusan
	// berganti, papan nama koperasi tidak.
	Phone     *string   `gorm:"column:phone"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (Cooperative) TableName() string { return "cooperative" }

type AppUser struct {
	ID            string             `gorm:"column:id;primaryKey"`
	Role          constants.UserRole `gorm:"column:role;type:user_role"`
	CooperativeID *string            `gorm:"column:cooperative_id"`
	FullName      string             `gorm:"column:full_name"`
	Organisation  *string            `gorm:"column:organisation"`
	// Phone dipakai untuk menyusun tautan WhatsApp, bukan untuk autentikasi.
	// Null pada akun yang dibuat sebelum kolomnya ada.
	Phone     *string   `gorm:"column:phone"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (AppUser) TableName() string { return "app_user" }

type Member struct {
	ID            string    `gorm:"column:id;primaryKey"`
	CooperativeID string    `gorm:"column:cooperative_id"`
	Name          string    `gorm:"column:name"`
	Phone         *string   `gorm:"column:phone"`
	NIKHash       *string   `gorm:"column:nik_hash"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (Member) TableName() string { return "member" }

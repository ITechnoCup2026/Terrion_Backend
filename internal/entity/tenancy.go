package entity

import (
	"encoding/json"
	"time"

	"terrion-backend/internal/constants"
)

type Cooperative struct {
	ID           string  `gorm:"column:id;primaryKey;default:gen_random_uuid()"`
	Name         string  `gorm:"column:name"`
	Village      string  `gorm:"column:village"`
	District     string  `gorm:"column:district"`
	DistrictCode *string `gorm:"column:district_code"`
	Province     string  `gorm:"column:province"`
	Lat          float64 `gorm:"column:lat"`
	Lng          float64 `gorm:"column:lng"`
	// Every staggering suggestion this cooperative has accepted, appended never
	// replaced: impact figure 4 reads all of them to reconstruct the season
	// that would have happened without them.
	StaggerApplied json.RawMessage `gorm:"column:stagger_applied;type:jsonb"`
	CreatedAt      time.Time       `gorm:"column:created_at"`
}

func (Cooperative) TableName() string { return "cooperative" }

// AppUser is the profile behind a Supabase auth user. Its ID is auth.users.id,
// so it is never generated here — it arrives with the verified access token.
type AppUser struct {
	ID            string             `gorm:"column:id;primaryKey"`
	Role          constants.UserRole `gorm:"column:role;type:user_role"`
	CooperativeID *string            `gorm:"column:cooperative_id"`
	FullName      string             `gorm:"column:full_name"`
	Organisation  *string            `gorm:"column:organisation"`
	CreatedAt     time.Time          `gorm:"column:created_at"`
}

func (AppUser) TableName() string { return "app_user" }

type Member struct {
	ID            string    `gorm:"column:id;primaryKey;default:gen_random_uuid()"`
	CooperativeID string    `gorm:"column:cooperative_id"`
	Name          string    `gorm:"column:name"`
	NIKHash       *string   `gorm:"column:nik_hash"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

func (Member) TableName() string { return "member" }

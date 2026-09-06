package repository

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
)

func planShareTokenDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("opening in-memory sqlite: %v", err)
	}

	pool, err := db.DB()
	if err != nil {
		t.Fatalf("reading sqlite connection pool: %v", err)
	}
	pool.SetMaxOpenConns(1)

	if err := db.AutoMigrate(&entity.PlanShareToken{}); err != nil {
		t.Fatalf("migrating plan_share_token: %v", err)
	}
	return db
}

func TestPlanShareTokenRejectsDuplicatePlanAndMember(t *testing.T) {
	db := planShareTokenDB(t)
	repo := &PlanShareTokenRepository{}

	first := &entity.PlanShareToken{ID: "token-1", PlanID: "plan-1", MemberID: "member-1"}
	if err := repo.Create(db, first); err != nil {
		t.Fatalf("Create first: %v", err)
	}

	second := &entity.PlanShareToken{ID: "token-2", PlanID: "plan-1", MemberID: "member-1"}
	if err := repo.Create(db, second); err == nil {
		t.Fatal("Create second: want an error for a duplicate (plan_id, member_id), got nil")
	}
}

func TestFindByPlanIDReturnsOnlyThatPlansTokens(t *testing.T) {
	db := planShareTokenDB(t)
	repo := &PlanShareTokenRepository{}

	seed := []entity.PlanShareToken{
		{ID: "token-1", PlanID: "plan-1", MemberID: "member-1"},
		{ID: "token-2", PlanID: "plan-1", MemberID: "member-2"},
		{ID: "token-3", PlanID: "plan-2", MemberID: "member-3"},
	}
	for i := range seed {
		if err := repo.Create(db, &seed[i]); err != nil {
			t.Fatalf("seeding %s: %v", seed[i].ID, err)
		}
	}

	found, err := repo.FindByPlanID(db, "plan-1")
	if err != nil {
		t.Fatalf("FindByPlanID: %v", err)
	}
	if len(found) != 2 {
		t.Fatalf("len(found) = %d, want 2", len(found))
	}
}

func TestMarkViewedSetsFirstViewOnceAndLastViewEveryTime(t *testing.T) {
	db := planShareTokenDB(t)
	repo := &PlanShareTokenRepository{}

	if err := repo.Create(db, &entity.PlanShareToken{
		ID: "token-1", PlanID: "plan-1", MemberID: "member-1",
	}); err != nil {
		t.Fatalf("seeding token: %v", err)
	}

	first := time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)
	if err := repo.MarkViewed(db, "token-1", first); err != nil {
		t.Fatalf("MarkViewed (first): %v", err)
	}

	loaded := new(entity.PlanShareToken)
	if err := repo.FindById(db, loaded, "token-1"); err != nil {
		t.Fatalf("FindById: %v", err)
	}
	if loaded.FirstViewedAt == nil || !loaded.FirstViewedAt.Equal(first) {
		t.Fatalf("FirstViewedAt = %v, want %v", loaded.FirstViewedAt, first)
	}

	second := first.Add(24 * time.Hour)
	if err := repo.MarkViewed(db, "token-1", second); err != nil {
		t.Fatalf("MarkViewed (second): %v", err)
	}

	loaded = new(entity.PlanShareToken)
	if err := repo.FindById(db, loaded, "token-1"); err != nil {
		t.Fatalf("FindById: %v", err)
	}
	if !loaded.FirstViewedAt.Equal(first) {
		t.Errorf("FirstViewedAt = %v, want unchanged %v", loaded.FirstViewedAt, first)
	}
	if loaded.LastViewedAt == nil || !loaded.LastViewedAt.Equal(second) {
		t.Errorf("LastViewedAt = %v, want %v", loaded.LastViewedAt, second)
	}
}

package repository

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type testItem struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite: %v", err)
	}

	if err := db.AutoMigrate(&testItem{}); err != nil {
		t.Fatalf("failed to migrate testItem: %v", err)
	}

	return db
}

func TestRepository_CreateAndFindById(t *testing.T) {
	db := setupTestDB(t)
	repo := &Repository[testItem]{}

	item := &testItem{Name: "widget"}
	if err := repo.Create(db, item); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if item.ID == 0 {
		t.Fatal("expected ID to be set after Create")
	}

	found := new(testItem)
	if err := repo.FindById(db, found, item.ID); err != nil {
		t.Fatalf("FindById error: %v", err)
	}
	if found.Name != "widget" {
		t.Errorf("found.Name = %q, want %q", found.Name, "widget")
	}
}

func TestRepository_UpdateDeleteAndCount(t *testing.T) {
	db := setupTestDB(t)
	repo := &Repository[testItem]{}

	item := &testItem{Name: "widget"}
	if err := repo.Create(db, item); err != nil {
		t.Fatalf("Create error: %v", err)
	}

	item.Name = "updated-widget"
	if err := repo.Update(db, item); err != nil {
		t.Fatalf("Update error: %v", err)
	}

	total, err := repo.CountById(db, item.ID)
	if err != nil {
		t.Fatalf("CountById error: %v", err)
	}
	if total != 1 {
		t.Errorf("CountById = %d, want 1", total)
	}

	if err := repo.Delete(db, item); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	total, err = repo.CountById(db, item.ID)
	if err != nil {
		t.Fatalf("CountById after delete error: %v", err)
	}
	if total != 0 {
		t.Errorf("CountById after delete = %d, want 0", total)
	}
}

package database

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"app/internal/config"

	"golang.org/x/crypto/bcrypt"
)

func TestConnectDatabase_SQLiteMemoryAndMigrateAndSeed(t *testing.T) {
	cfg := &config.Config{
		DBConnection:  "sqlite",
		DBDatabase:    ":memory:",
		AdminUsername: "admin",
		AdminPassword: "admin",
	}

	db, err := ConnectDatabase(cfg)
	if err != nil || db == nil {
		t.Fatalf("expected no error connecting sqlite, got: %v", err)
	}

	if err := AutoMigrate(db); err != nil {
		t.Fatalf("expected no error on AutoMigrate, got: %v", err)
	}

	if err := Seed(db, cfg); err != nil {
		t.Fatalf("expected no error on Seed, got: %v", err)
	}

	// Re-run seed with nil config to verify idempotency and default fallback
	if err := Seed(db, nil); err != nil {
		t.Fatalf("expected no error on second Seed, got: %v", err)
	}

	var admin User
	if err := db.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("expected admin user, got error: %v", err)
	}
	if admin.Role != RoleAdmin || admin.Email != "admin@delcom.org" {
		t.Fatalf("unexpected admin data: %+v", admin)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte("admin")); err != nil {
		t.Fatalf("admin password hash mismatch: %v", err)
	}

	var member User
	if err := db.Where("username = ?", "member").First(&member).Error; err != nil {
		t.Fatalf("expected member user, got error: %v", err)
	}
	if member.Role != RoleMember || member.Email != "member@delcom.org" {
		t.Fatalf("unexpected member data: %+v", member)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(member.Password), []byte("member")); err != nil {
		t.Fatalf("member password hash mismatch: %v", err)
	}

	// Verify Post creation and ON DELETE CASCADE with User
	now := time.Now()
	post := Post{
		UserID:         member.ID,
		Title:          "Belajar Golang Gin",
		Slug:           "belajar-golang-gin",
		Category:       "Programming",
		Excerpt:        "Ringkasan belajar Gin",
		Content:        "Konten lengkap belajar Golang Gin Clean Architecture",
		CoverImagePath: "/uploads/cover.jpg",
		IsPublished:    true,
		PublishedAt:    &now,
	}
	if err := db.Create(&post).Error; err != nil || post.ID == 0 {
		t.Fatalf("failed creating post: %v", err)
	}

	// Delete user and check cascade delete on post
	if err := db.Delete(&member).Error; err != nil {
		t.Fatalf("failed deleting member user: %v", err)
	}

	var count int64
	db.Model(&Post{}).Where("id = ?", post.ID).Count(&count)
	if count != 0 {
		t.Fatalf("expected post to be cascade deleted, count=%d", count)
	}
}

func TestConnectDatabase_SQLiteCustomDSN(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.db")
	cfg := &config.Config{
		DBConnection: "sqlite",
		DBDatabase:   tmpFile,
	}
	db, err := ConnectDatabase(cfg)
	if err != nil || db == nil {
		t.Fatalf("expected no error connecting custom sqlite file, got: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed getting sql.DB: %v", err)
	}
	_ = sqlDB.Close()
}

func TestConnectDatabase_Postgres(t *testing.T) {
	cfg := &config.Config{
		DBConnection: "pgsql",
		DBHost:       "127.0.0.1",
		DBPort:       "1",
		DBUsername:   "postgres",
		DBPassword:   "invalid",
		DBDatabase:   "non_existent_db",
		DBSSLMode:    "disable",
	}

	_, err := ConnectDatabase(cfg)
	if err == nil {
		t.Fatalf("expected error connecting to invalid postgres port")
	}
}

func TestSeed_Errors(t *testing.T) {
	cfg := &config.Config{
		DBConnection: "sqlite",
		DBDatabase:   "",
	}
	db, err := ConnectDatabase(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 1. Seed before AutoMigrate -> DB error on FirstOrCreate
	if err := Seed(db, cfg); err == nil {
		t.Fatalf("expected error when seeding before migration")
	}

	// 2. Seed with password > 72 bytes -> bcrypt error
	longPasswordCfg := &config.Config{
		AdminUsername: "admin",
		AdminPassword: strings.Repeat("a", 73),
	}
	if err := Seed(db, longPasswordCfg); err == nil {
		t.Fatalf("expected bcrypt error when password exceeds 72 bytes")
	}
}

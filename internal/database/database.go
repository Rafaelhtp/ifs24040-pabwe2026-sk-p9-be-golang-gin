package database

import (
	"fmt"
	"strings"
	"time"

	"app/internal/config"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	RoleAdmin  = "admin"
	RoleMember = "member"
)

type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Username  string    `gorm:"type:varchar(100);uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"type:varchar(150);uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"type:varchar(255);not null" json:"-"`
	Role      string    `gorm:"type:varchar(20);not null;default:'member'" json:"role"`
	Posts     []Post    `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"posts,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Post struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	UserID         uint       `gorm:"not null;index" json:"user_id"`
	User           User       `gorm:"foreignKey:UserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"user,omitempty"`
	Title          string     `gorm:"type:varchar(255);not null" json:"title"`
	Slug           string     `gorm:"type:varchar(255);uniqueIndex;not null" json:"slug"`
	Category       string     `gorm:"type:varchar(100);not null" json:"category"`
	Excerpt        string     `gorm:"type:text" json:"excerpt"`
	Content        string     `gorm:"type:text;not null" json:"content"`
	CoverImagePath string     `gorm:"type:varchar(255)" json:"cover_image_path"`
	IsPublished    bool       `gorm:"default:false;not null" json:"is_published"`
	PublishedAt    *time.Time `json:"published_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func ConnectDatabase(cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector

	if cfg.DBConnection == "sqlite" {
		dsn := cfg.DBDatabase
		if dsn == "" || dsn == ":memory:" {
			dsn = "file::memory:?_pragma=foreign_keys(1)"
		} else if !strings.Contains(dsn, "_pragma=foreign_keys") {
			dsn = dsn + "?_pragma=foreign_keys(1)"
		}
		dialector = sqlite.Open(dsn)
	} else {
		dsn := fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
			cfg.DBHost,
			cfg.DBUsername,
			cfg.DBPassword,
			cfg.DBDatabase,
			cfg.DBPort,
			cfg.DBSSLMode,
		)
		dialector = postgres.Open(dsn)
	}

	return gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
}

func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&User{}, &Post{})
}

func Seed(db *gorm.DB, cfg *config.Config) error {
	adminUsername := "admin"
	adminPassword := "admin"
	if cfg != nil {
		if cfg.AdminUsername != "" {
			adminUsername = cfg.AdminUsername
		}
		if cfg.AdminPassword != "" {
			adminPassword = cfg.AdminPassword
		}
	}

	seedUsers := []struct {
		Username string
		Email    string
		Password string
		Role     string
	}{
		{
			Username: adminUsername,
			Email:    adminUsername + "@delcom.org",
			Password: adminPassword,
			Role:     RoleAdmin,
		},
		{
			Username: "member",
			Email:    "member@delcom.org",
			Password: "member",
			Role:     RoleMember,
		},
	}

	for _, item := range seedUsers {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(item.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		user := User{
			Username: item.Username,
			Email:    item.Email,
			Password: string(hashedPassword),
			Role:     item.Role,
		}

		if err := db.Where("username = ?", item.Username).FirstOrCreate(&user).Error; err != nil {
			return err
		}

		// Pastikan akun bawaan selalu memiliki role yang benar (mis. jika sudah terdaftar sebagai member).
		if user.Role != item.Role {
			if err := db.Model(&user).Update("role", item.Role).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

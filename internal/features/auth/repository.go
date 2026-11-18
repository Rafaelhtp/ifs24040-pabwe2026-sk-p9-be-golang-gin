package auth

import (
	"app/internal/database"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(user *database.User) error {
	return r.db.Create(user).Error
}

func (r *Repository) FindByUsernameOrEmail(identifier string) (*database.User, error) {
	var user database.User
	if err := r.db.Where("username = ? OR email = ?", identifier, identifier).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) ExistsByUsernameOrEmail(username, email string, excludeID uint) (bool, error) {
	var count int64
	query := r.db.Model(&database.User{}).Where("(username = ? OR email = ?)", username, email)
	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) FindByID(id uint) (*database.User, error) {
	var user database.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *Repository) Update(user *database.User) error {
	return r.db.Save(user).Error
}

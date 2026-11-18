package post

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

func (r *Repository) Create(p *database.Post) error {
	return r.db.Create(p).Error
}

func (r *Repository) Save(p *database.Post) error {
	return r.db.Save(p).Error
}

func (r *Repository) Delete(p *database.Post) error {
	return r.db.Delete(p).Error
}

func (r *Repository) SlugExists(slug string, excludeID uint) (bool, error) {
	var count int64
	q := r.db.Model(&database.Post{}).Where("slug = ?", slug)
	if excludeID > 0 {
		q = q.Where("id != ?", excludeID)
	}
	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) FindByIDAndUser(id, userID uint) (*database.Post, error) {
	var p database.Post
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) FindPublishedBySlug(slug string) (*database.Post, error) {
	var p database.Post
	if err := r.db.Preload("User").Where("slug = ? AND is_published = ?", slug, true).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) ListPublished(f PostFilter) ([]database.Post, error) {
	var posts []database.Post
	q := r.db.Preload("User").Where("is_published = ?", true)
	if f.Query != "" {
		like := "%" + f.Query + "%"
		q = q.Where("title LIKE ? OR excerpt LIKE ? OR content LIKE ?", like, like, like)
	}
	if f.Category != "" {
		q = q.Where("category = ?", f.Category)
	}
	if err := q.Order("published_at DESC, id DESC").Find(&posts).Error; err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *Repository) ListByUser(userID uint) ([]database.Post, error) {
	var posts []database.Post
	if err := r.db.Where("user_id = ?", userID).Order("id DESC").Find(&posts).Error; err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *Repository) CountByUser(userID uint) (total, published int64, err error) {
	if err = r.db.Model(&database.Post{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return
	}
	err = r.db.Model(&database.Post{}).Where("user_id = ? AND is_published = ?", userID, true).Count(&published).Error
	return
}

func (r *Repository) ListAll() ([]database.Post, error) {
	var posts []database.Post
	if err := r.db.Preload("User").Order("id DESC").Find(&posts).Error; err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *Repository) ListUsers() ([]database.User, error) {
	var users []database.User
	if err := r.db.Order("id ASC").Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

func (r *Repository) UserExists(id uint) (bool, error) {
	var count int64
	if err := r.db.Model(&database.User{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

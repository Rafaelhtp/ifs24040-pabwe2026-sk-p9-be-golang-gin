package post

import (
	"time"

	"app/internal/database"
)

var Categories = []string{"Teknologi", "Pendidikan", "Gaya Hidup", "Bisnis", "Opini", "Umum"}

type PostRequest struct {
	Title          string `json:"title" binding:"required"`
	Category       string `json:"category" binding:"required"`
	Excerpt        string `json:"excerpt"`
	Content        string `json:"content" binding:"required"`
	CoverImagePath string `json:"cover_image_path"`
	IsPublished    bool   `json:"is_published"`
}

type AuthorResponse struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
}

type PostResponse struct {
	ID             uint            `json:"id"`
	UserID         uint            `json:"user_id"`
	Title          string          `json:"title"`
	Slug           string          `json:"slug"`
	Category       string          `json:"category"`
	Excerpt        string          `json:"excerpt"`
	Content        string          `json:"content"`
	CoverImagePath string          `json:"cover_image_path"`
	IsPublished    bool            `json:"is_published"`
	PublishedAt    *time.Time      `json:"published_at"`
	ReadingTime    int             `json:"reading_time"`
	Author         *AuthorResponse `json:"author,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type StatsResponse struct {
	TotalPosts     int64 `json:"total_posts"`
	TotalPublished int64 `json:"total_published"`
	TotalDraft     int64 `json:"total_draft"`
}

type AdminUserResponse struct {
	ID             uint      `json:"id"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	TotalPosts     int64     `json:"total_posts"`
	TotalPublished int64     `json:"total_published"`
	CreatedAt      time.Time `json:"created_at"`
}

type PostFilter struct {
	Query    string
	Category string
}

func ToPostResponse(p database.Post, withAuthor bool) PostResponse {
	res := PostResponse{
		ID:             p.ID,
		UserID:         p.UserID,
		Title:          p.Title,
		Slug:           p.Slug,
		Category:       p.Category,
		Excerpt:        p.Excerpt,
		Content:        p.Content,
		CoverImagePath: p.CoverImagePath,
		IsPublished:    p.IsPublished,
		PublishedAt:    p.PublishedAt,
		ReadingTime:    CalculateReadingTime(p.Content),
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
	if withAuthor {
		res.Author = &AuthorResponse{ID: p.User.ID, Username: p.User.Username}
	}
	return res
}

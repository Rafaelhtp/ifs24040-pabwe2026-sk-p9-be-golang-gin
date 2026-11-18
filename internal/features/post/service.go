package post

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"app/internal/database"
)

var (
	ErrPostNotFound     = errors.New("artikel tidak ditemukan")
	ErrUserNotFound     = errors.New("user tidak ditemukan")
	ErrInvalidCategory  = errors.New("kategori tidak valid")
	nonAlphanumericRegx = regexp.MustCompile(`[^a-z0-9]+`)
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func CalculateReadingTime(content string) int {
	minutes := len(strings.Fields(content)) / 200
	if minutes < 1 {
		return 1
	}
	return minutes
}

func Slugify(title string) string {
	s := nonAlphanumericRegx.ReplaceAllString(strings.ToLower(title), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "artikel"
	}
	return s
}

func validCategory(category string) bool {
	for _, c := range Categories {
		if c == category {
			return true
		}
	}
	return false
}

func (s *Service) uniqueSlug(title string, excludeID uint) (string, error) {
	base := Slugify(title)
	slug := base
	for i := 2; ; i++ {
		exists, err := s.repo.SlugExists(slug, excludeID)
		if err != nil {
			return "", err
		}
		if !exists {
			return slug, nil
		}
		slug = base + "-" + strconv.Itoa(i)
	}
}

func (s *Service) GetCategories() []string {
	return Categories
}

func (s *Service) ListPublic(f PostFilter) ([]PostResponse, error) {
	posts, err := s.repo.ListPublished(f)
	if err != nil {
		return nil, err
	}
	return toResponses(posts, true), nil
}

func (s *Service) GetPublicBySlug(slug string) (*PostResponse, error) {
	p, err := s.repo.FindPublishedBySlug(slug)
	if err != nil {
		return nil, ErrPostNotFound
	}
	res := ToPostResponse(*p, true)
	return &res, nil
}

func (s *Service) Create(userID uint, req PostRequest) (*PostResponse, error) {
	if !validCategory(req.Category) {
		return nil, ErrInvalidCategory
	}
	slug, err := s.uniqueSlug(req.Title, 0)
	if err != nil {
		return nil, err
	}
	p := database.Post{
		UserID:         userID,
		Title:          req.Title,
		Slug:           slug,
		Category:       req.Category,
		Excerpt:        req.Excerpt,
		Content:        req.Content,
		CoverImagePath: req.CoverImagePath,
		IsPublished:    req.IsPublished,
	}
	if req.IsPublished {
		now := time.Now()
		p.PublishedAt = &now
	}
	if err := s.repo.Create(&p); err != nil {
		return nil, err
	}
	res := ToPostResponse(p, false)
	return &res, nil
}

func (s *Service) ListMine(userID uint) ([]PostResponse, error) {
	posts, err := s.repo.ListByUser(userID)
	if err != nil {
		return nil, err
	}
	return toResponses(posts, false), nil
}

func (s *Service) GetMine(userID, id uint) (*PostResponse, error) {
	p, err := s.repo.FindByIDAndUser(id, userID)
	if err != nil {
		return nil, ErrPostNotFound
	}
	res := ToPostResponse(*p, false)
	return &res, nil
}

func (s *Service) Update(userID, id uint, req PostRequest) (*PostResponse, error) {
	p, err := s.repo.FindByIDAndUser(id, userID)
	if err != nil {
		return nil, ErrPostNotFound
	}
	if !validCategory(req.Category) {
		return nil, ErrInvalidCategory
	}
	if req.Title != p.Title {
		slug, err := s.uniqueSlug(req.Title, p.ID)
		if err != nil {
			return nil, err
		}
		p.Slug = slug
	}
	p.Title = req.Title
	p.Category = req.Category
	p.Excerpt = req.Excerpt
	p.Content = req.Content
	p.CoverImagePath = req.CoverImagePath
	applyPublish(p, req.IsPublished)
	if err := s.repo.Save(p); err != nil {
		return nil, err
	}
	res := ToPostResponse(*p, false)
	return &res, nil
}

func applyPublish(p *database.Post, publish bool) {
	if publish && !p.IsPublished {
		now := time.Now()
		p.PublishedAt = &now
	}
	if !publish {
		p.PublishedAt = nil
	}
	p.IsPublished = publish
}

func (s *Service) Delete(userID, id uint) error {
	p, err := s.repo.FindByIDAndUser(id, userID)
	if err != nil {
		return ErrPostNotFound
	}
	return s.repo.Delete(p)
}

func (s *Service) TogglePublish(userID, id uint) (*PostResponse, error) {
	p, err := s.repo.FindByIDAndUser(id, userID)
	if err != nil {
		return nil, ErrPostNotFound
	}
	applyPublish(p, !p.IsPublished)
	if err := s.repo.Save(p); err != nil {
		return nil, err
	}
	res := ToPostResponse(*p, false)
	return &res, nil
}

func (s *Service) Stats(userID uint) (*StatsResponse, error) {
	total, published, err := s.repo.CountByUser(userID)
	if err != nil {
		return nil, err
	}
	return &StatsResponse{
		TotalPosts:     total,
		TotalPublished: published,
		TotalDraft:     total - published,
	}, nil
}

func (s *Service) AdminListUsers() ([]AdminUserResponse, error) {
	users, err := s.repo.ListUsers()
	if err != nil {
		return nil, err
	}
	result := make([]AdminUserResponse, 0, len(users))
	for _, u := range users {
		total, published, err := s.repo.CountByUser(u.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, AdminUserResponse{
			ID:             u.ID,
			Username:       u.Username,
			Email:          u.Email,
			Role:           u.Role,
			TotalPosts:     total,
			TotalPublished: published,
			CreatedAt:      u.CreatedAt,
		})
	}
	return result, nil
}

func (s *Service) AdminListPosts() ([]PostResponse, error) {
	posts, err := s.repo.ListAll()
	if err != nil {
		return nil, err
	}
	return toResponses(posts, true), nil
}

func (s *Service) AdminListUserPosts(userID uint) ([]PostResponse, error) {
	exists, err := s.repo.UserExists(userID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrUserNotFound
	}
	return s.ListMine(userID)
}

func toResponses(posts []database.Post, withAuthor bool) []PostResponse {
	result := make([]PostResponse, 0, len(posts))
	for _, p := range posts {
		result = append(result, ToPostResponse(p, withAuthor))
	}
	return result
}

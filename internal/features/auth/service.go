package auth

import (
	"errors"
	"strings"
	"time"

	"app/internal/database"
	"app/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserAlreadyExists  = errors.New("username atau email sudah digunakan")
	ErrInvalidCredentials = errors.New("username/email atau password salah")
	ErrUserNotFound       = errors.New("user tidak ditemukan")
)

type Service struct {
	repo      *Repository
	secretKey string
}

func NewService(repo *Repository, secretKey string) *Service {
	return &Service{
		repo:      repo,
		secretKey: secretKey,
	}
}

func (s *Service) Register(req RegisterRequest) (*UserResponse, error) {
	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(strings.ToLower(req.Email))

	exists, err := s.repo.ExistsByUsernameOrEmail(username, email, 0)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUserAlreadyExists
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := database.User{
		Username: username,
		Email:    email,
		Password: string(hashed),
		Role:     database.RoleMember,
	}

	if err := s.repo.Create(&user); err != nil {
		return nil, err
	}

	res := ToUserResponse(user)
	return &res, nil
}

func (s *Service) Login(req LoginRequest) (*AuthResponse, error) {
	identifier := strings.TrimSpace(req.Username)
	user, err := s.repo.FindByUsernameOrEmail(identifier)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	claims := &middleware.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Email:    user.Email,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(s.secretKey))

	return &AuthResponse{
		Token: tokenString,
		User:  ToUserResponse(*user),
	}, nil
}

func (s *Service) GetProfile(userID uint) (*UserResponse, error) {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, ErrUserNotFound
	}
	res := ToUserResponse(*user)
	return &res, nil
}

func (s *Service) UpdateProfile(userID uint, req UpdateProfileRequest) (*UserResponse, error) {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	username := strings.TrimSpace(req.Username)
	email := strings.TrimSpace(strings.ToLower(req.Email))

	exists, err := s.repo.ExistsByUsernameOrEmail(username, email, userID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrUserAlreadyExists
	}

	user.Username = username
	user.Email = email

	if strings.TrimSpace(req.Password) != "" {
		hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		user.Password = string(hashed)
	}

	if err := s.repo.Update(user); err != nil {
		return nil, err
	}

	res := ToUserResponse(*user)
	return &res, nil
}

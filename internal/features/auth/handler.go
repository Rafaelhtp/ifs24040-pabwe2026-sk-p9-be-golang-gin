package auth

import (
	"errors"
	"net/http"

	"app/internal/response"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(api *gin.RouterGroup, authMiddleware gin.HandlerFunc) {
	authGroup := api.Group("/auth")
	{
		authGroup.POST("/register", h.Register)
		authGroup.POST("/login", h.Login)
		authGroup.POST("/logout", authMiddleware, h.Logout)
		authGroup.GET("/profile", authMiddleware, h.GetProfile)
		authGroup.PUT("/profile", authMiddleware, h.UpdateProfile)
	}
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data registrasi tidak valid: "+err.Error())
		return
	}

	res, err := h.service.Register(req)
	if err != nil {
		if errors.Is(err, ErrUserAlreadyExists) {
			response.Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "Gagal melakukan registrasi: "+err.Error())
		return
	}

	response.Success(c, http.StatusCreated, "Registrasi berhasil", res)
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data login tidak valid: "+err.Error())
		return
	}

	res, err := h.service.Login(req)
	if err != nil {
		response.Fail(c, http.StatusUnauthorized, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Login berhasil", res)
}

func (h *Handler) Logout(c *gin.Context) {
	response.Success(c, http.StatusOK, "Logout berhasil")
}

func (h *Handler) GetProfile(c *gin.Context) {
	userID := c.GetUint("user_id")
	res, err := h.service.GetProfile(userID)
	if err != nil {
		response.Fail(c, http.StatusNotFound, err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Berhasil mengambil profil", res)
}

func (h *Handler) UpdateProfile(c *gin.Context) {
	userID := c.GetUint("user_id")
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data profil tidak valid: "+err.Error())
		return
	}

	res, err := h.service.UpdateProfile(userID, req)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			response.Fail(c, http.StatusNotFound, err.Error())
			return
		}
		if errors.Is(err, ErrUserAlreadyExists) {
			response.Fail(c, http.StatusConflict, err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "Gagal memperbarui profil: "+err.Error())
		return
	}

	response.Success(c, http.StatusOK, "Profil berhasil diperbarui", res)
}

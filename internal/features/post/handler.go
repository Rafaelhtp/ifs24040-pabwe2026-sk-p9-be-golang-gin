package post

import (
	"errors"
	"net/http"
	"strconv"

	"app/internal/middleware"
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
	public := api.Group("/public")
	public.GET("/categories", h.Categories)
	public.GET("/posts", h.ListPublic)
	public.GET("/posts/:slug", h.DetailPublic)

	posts := api.Group("/posts", authMiddleware)
	posts.GET("", h.ListMine)
	posts.POST("", h.Create)
	posts.GET("/stats", h.Stats)
	posts.GET("/:id", h.GetMine)
	posts.PUT("/:id", h.Update)
	posts.DELETE("/:id", h.Delete)
	posts.PATCH("/:id/toggle-publish", h.TogglePublish)

	admin := api.Group("/admin", authMiddleware, middleware.RequireRole("admin"))
	admin.GET("/users", h.AdminUsers)
	admin.GET("/posts", h.AdminPosts)
	admin.GET("/users/:id/posts", h.AdminUserPosts)
}

func parseID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.Fail(c, http.StatusBadRequest, "ID tidak valid")
		return 0, false
	}
	return uint(id), true
}

func (h *Handler) handleErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrPostNotFound), errors.Is(err, ErrUserNotFound):
		response.Fail(c, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrInvalidCategory):
		response.Fail(c, http.StatusBadRequest, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, err.Error())
	}
}

func (h *Handler) Categories(c *gin.Context) {
	response.Success(c, http.StatusOK, "Berhasil mengambil kategori", h.service.GetCategories())
}

func (h *Handler) ListPublic(c *gin.Context) {
	res, err := h.service.ListPublic(PostFilter{Query: c.Query("q"), Category: c.Query("category")})
	if err != nil {
		h.handleErr(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Berhasil mengambil artikel", res)
}

func (h *Handler) DetailPublic(c *gin.Context) {
	res, err := h.service.GetPublicBySlug(c.Param("slug"))
	if err != nil {
		h.handleErr(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Berhasil mengambil detail artikel", res)
}

func (h *Handler) ListMine(c *gin.Context) {
	res, err := h.service.ListMine(c.GetUint("user_id"))
	if err != nil {
		h.handleErr(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Berhasil mengambil artikel", res)
}

func (h *Handler) Create(c *gin.Context) {
	var req PostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data artikel tidak valid: "+err.Error())
		return
	}
	res, err := h.service.Create(c.GetUint("user_id"), req)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	response.Success(c, http.StatusCreated, "Artikel berhasil dibuat", res)
}

func (h *Handler) GetMine(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	res, err := h.service.GetMine(c.GetUint("user_id"), id)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Berhasil mengambil artikel", res)
}

func (h *Handler) Update(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	var req PostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, http.StatusBadRequest, "Data artikel tidak valid: "+err.Error())
		return
	}
	res, err := h.service.Update(c.GetUint("user_id"), id, req)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil diperbarui", res)
}

func (h *Handler) Delete(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(c.GetUint("user_id"), id); err != nil {
		h.handleErr(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Artikel berhasil dihapus")
}

func (h *Handler) TogglePublish(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	res, err := h.service.TogglePublish(c.GetUint("user_id"), id)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Status publikasi berhasil diubah", res)
}

func (h *Handler) Stats(c *gin.Context) {
	res, err := h.service.Stats(c.GetUint("user_id"))
	if err != nil {
		h.handleErr(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Berhasil mengambil statistik", res)
}

func (h *Handler) AdminUsers(c *gin.Context) {
	res, err := h.service.AdminListUsers()
	if err != nil {
		h.handleErr(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Berhasil mengambil daftar user", res)
}

func (h *Handler) AdminPosts(c *gin.Context) {
	res, err := h.service.AdminListPosts()
	if err != nil {
		h.handleErr(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Berhasil mengambil seluruh artikel", res)
}

func (h *Handler) AdminUserPosts(c *gin.Context) {
	id, ok := parseID(c)
	if !ok {
		return
	}
	res, err := h.service.AdminListUserPosts(id)
	if err != nil {
		h.handleErr(c, err)
		return
	}
	response.Success(c, http.StatusOK, "Berhasil mengambil artikel user", res)
}

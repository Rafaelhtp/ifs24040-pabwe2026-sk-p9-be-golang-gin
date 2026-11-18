package server

import (
	"net/http"

	"app/docs"
	"app/internal/config"
	"app/internal/features/ai"
	"app/internal/features/auth"
	"app/internal/features/post"
	"app/internal/middleware"
	"app/internal/response"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

func requestScheme(c *gin.Context) string {
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	if c.Request.TLS != nil {
		return "https"
	}
	return "http"
}

func NewRouter(db *gorm.DB, cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORS())
	r.Use(middleware.RateLimiter())

	api := r.Group("/api")
	authMiddleware := middleware.Auth(cfg.AppKey)

	authService := auth.NewService(auth.NewRepository(db), cfg.AppKey)
	auth.NewHandler(authService).RegisterRoutes(api, authMiddleware)

	post.NewHandler(post.NewService(post.NewRepository(db))).RegisterRoutes(api, authMiddleware)

	aiService := ai.NewService(
		ai.NewSearchClient(cfg.SearchEngineEndpoint),
		ai.NewOpenAIClient(cfg.OpenAIEndpoint, cfg.OpenAIKey, cfg.OpenAIModel),
	)
	ai.NewHandler(aiService).RegisterRoutes(api, authMiddleware)

	r.GET("/api.json", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", docs.GetSwaggerJSON(c.Request.Host, requestScheme(c)))
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.NoRoute(func(c *gin.Context) {
		response.Fail(c, http.StatusNotFound, "Endpoint not found")
	})

	return r
}

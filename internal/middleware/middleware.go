package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"app/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, Accept, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func Auth(secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			response.AbortFail(c, http.StatusUnauthorized, "Token autentikasi tidak ditemukan atau format tidak valid")
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if tokenString == "" {
			response.AbortFail(c, http.StatusUnauthorized, "Token autentikasi tidak boleh kosong")
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secretKey), nil
		})

		if err != nil || !token.Valid {
			response.AbortFail(c, http.StatusUnauthorized, "Token tidak valid atau telah kedaluwarsa")
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Set("claims", claims)

		c.Next()
	}
}

func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := c.GetString("role")
		if userRole != "" {
			for _, role := range roles {
				if strings.EqualFold(strings.TrimSpace(userRole), strings.TrimSpace(role)) {
					c.Next()
					return
				}
			}
		}

		response.AbortFail(c, http.StatusForbidden, "Akses ditolak: role tidak memiliki izin")
	}
}

type clientLimiter struct {
	count     int
	expiresAt time.Time
}

func RateLimiter(options ...int) gin.HandlerFunc {
	maxRequests := 1000
	window := time.Minute

	if len(options) > 0 && options[0] > 0 {
		maxRequests = options[0]
	}
	if len(options) > 1 && options[1] > 0 {
		window = time.Duration(options[1]) * time.Millisecond
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*clientLimiter)
	)

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		mu.Lock()
		limiter, exists := clients[ip]
		if !exists || now.After(limiter.expiresAt) {
			clients[ip] = &clientLimiter{
				count:     1,
				expiresAt: now.Add(window),
			}
			mu.Unlock()
			c.Next()
			return
		}

		limiter.count++
		if limiter.count > maxRequests {
			mu.Unlock()
			response.AbortFail(c, http.StatusTooManyRequests, "Terlalu banyak permintaan, silakan coba lagi nanti")
			return
		}
		mu.Unlock()

		c.Next()
	}
}

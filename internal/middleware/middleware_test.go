package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestCORS(t *testing.T) {
	r := gin.New()
	r.Use(CORS())
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// 1. Preflight OPTIONS request
	reqOptions := httptest.NewRequest(http.MethodOptions, "/ping", nil)
	wOptions := httptest.NewRecorder()
	r.ServeHTTP(wOptions, reqOptions)

	if wOptions.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", wOptions.Code)
	}
	if wOptions.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("missing CORS origin header")
	}
	if !strings.Contains(wOptions.Header().Get("Access-Control-Allow-Methods"), "OPTIONS") {
		t.Fatalf("missing OPTIONS in allow methods")
	}

	// 2. Normal GET request
	reqGet := httptest.NewRequest(http.MethodGet, "/ping", nil)
	wGet := httptest.NewRecorder()
	r.ServeHTTP(wGet, reqGet)

	if wGet.Code != http.StatusOK || wGet.Body.String() != "pong" {
		t.Fatalf("expected 200 pong, got %d %q", wGet.Code, wGet.Body.String())
	}
}

func TestAuth(t *testing.T) {
	secret := "test_secret_key"

	r := gin.New()
	r.Use(Auth(secret))
	r.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"user_id":  c.GetUint("user_id"),
			"username": c.GetString("username"),
			"email":    c.GetString("email"),
			"role":     c.GetString("role"),
		})
	})

	// 1. Missing Authorization header
	req1 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w1.Code)
	}

	// 2. Empty Bearer token
	req2 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req2.Header.Set("Authorization", "Bearer    ")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w2.Code)
	}

	// 3. Invalid token string
	req3 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req3.Header.Set("Authorization", "Bearer invalid.token.string")
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w3.Code)
	}

	// 4. Token signed with non-HMAC algorithm (RSA)
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed generating rsa key: %v", err)
	}
	rsaToken := jwt.NewWithClaims(jwt.SigningMethodRS256, &Claims{
		UserID:   1,
		Username: "admin",
		Role:     "admin",
	})
	rsaTokenStr, err := rsaToken.SignedString(rsaKey)
	if err != nil {
		t.Fatalf("failed signing rsa token: %v", err)
	}

	req4 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req4.Header.Set("Authorization", "Bearer "+rsaTokenStr)
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w4.Code)
	}

	// 5. Valid HMAC token
	validToken := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		UserID:   42,
		Username: "rafael",
		Email:    "rafael@delcom.org",
		Role:     "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	})
	validTokenStr, err := validToken.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed signing hmac token: %v", err)
	}

	req5 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req5.Header.Set("Authorization", "Bearer "+validTokenStr)
	w5 := httptest.NewRecorder()
	r.ServeHTTP(w5, req5)
	if w5.Code != http.StatusOK || !strings.Contains(w5.Body.String(), "rafael") {
		t.Fatalf("expected 200 with rafael, got %d %s", w5.Code, w5.Body.String())
	}
}

func TestRequireRole(t *testing.T) {
	r := gin.New()
	r.GET("/admin-only", func(c *gin.Context) {
		role := c.Query("role")
		if role != "" {
			c.Set("role", role)
		}
		c.Next()
	}, RequireRole("admin"), func(c *gin.Context) {
		c.String(http.StatusOK, "welcome admin")
	})

	// 1. No role in context
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest(http.MethodGet, "/admin-only", nil))
	if w1.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w1.Code)
	}

	// 2. Role is member (not allowed)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/admin-only?role=member", nil))
	if w2.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w2.Code)
	}

	// 3. Role is admin (allowed), case-insensitive
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, httptest.NewRequest(http.MethodGet, "/admin-only?role=Admin", nil))
	if w3.Code != http.StatusOK || w3.Body.String() != "welcome admin" {
		t.Fatalf("expected 200 welcome admin, got %d %s", w3.Code, w3.Body.String())
	}
}

func TestRateLimiter(t *testing.T) {
	// Test default RateLimiter() initialization without options
	defaultRouter := gin.New()
	defaultRouter.Use(RateLimiter())
	defaultRouter.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	wDef := httptest.NewRecorder()
	defaultRouter.ServeHTTP(wDef, httptest.NewRequest(http.MethodGet, "/test", nil))
	if wDef.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", wDef.Code)
	}

	// Custom RateLimiter: max 2 requests per 50ms window
	r := gin.New()
	r.Use(RateLimiter(2, 50))
	r.GET("/limited", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Request 1 -> 200 OK
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req1.RemoteAddr = "192.0.2.1:1234"
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200 on req 1, got %d", w1.Code)
	}

	// Request 2 -> 200 OK
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req2.RemoteAddr = "192.0.2.1:1234"
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 on req 2, got %d", w2.Code)
	}

	// Request 3 -> 429 Too Many Requests
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req3.RemoteAddr = "192.0.2.1:1234"
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on req 3, got %d", w3.Code)
	}

	// Wait for window to expire -> 200 OK again
	time.Sleep(70 * time.Millisecond)
	w4 := httptest.NewRecorder()
	req4 := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req4.RemoteAddr = "192.0.2.1:1234"
	r.ServeHTTP(w4, req4)
	if w4.Code != http.StatusOK {
		t.Fatalf("expected 200 after window reset, got %d", w4.Code)
	}
}

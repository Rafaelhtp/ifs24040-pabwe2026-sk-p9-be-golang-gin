package server

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"app/internal/config"
	"app/internal/database"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestRouter(t *testing.T) (*gin.Engine, *config.Config) {
	t.Helper()
	cfg := &config.Config{
		DBConnection: "sqlite",
		DBDatabase:   ":memory:",
		AppKey:       "router_test_secret",
	}
	db, err := database.ConnectDatabase(cfg)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := database.Seed(db, cfg); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return NewRouter(db, cfg), cfg
}

func do(r http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func login(t *testing.T, r http.Handler, username, password string) string {
	t.Helper()
	w := do(r, http.MethodPost, "/api/auth/login", "", map[string]string{"username": username, "password": password})
	if w.Code != http.StatusOK {
		t.Fatalf("login %s failed: %d %s", username, w.Code, w.Body.String())
	}
	var out struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return out.Data.Token
}

func expect(t *testing.T, w *httptest.ResponseRecorder, code int, label string) {
	t.Helper()
	if w.Code != code {
		t.Fatalf("%s: expected %d, got %d: %s", label, code, w.Code, w.Body.String())
	}
}

func TestRouter_MainRoutes(t *testing.T) {
	r, _ := newTestRouter(t)

	// public
	expect(t, do(r, http.MethodGet, "/api/public/categories", "", nil), 200, "categories")
	expect(t, do(r, http.MethodGet, "/api/public/posts", "", nil), 200, "public posts")
	expect(t, do(r, http.MethodGet, "/api/public/posts/tidak-ada", "", nil), 404, "public detail")

	// auth
	expect(t, do(r, http.MethodPost, "/api/auth/register", "", map[string]string{
		"username": "penulis", "email": "penulis@delcom.org", "password": "rahasia123"}), 201, "register")
	token := login(t, r, "penulis", "rahasia123")
	expect(t, do(r, http.MethodGet, "/api/auth/profile", token, nil), 200, "profile")
	expect(t, do(r, http.MethodPost, "/api/auth/logout", token, nil), 200, "logout")
	expect(t, do(r, http.MethodGet, "/api/auth/profile", "", nil), 401, "profile unauthenticated")

	// posts
	w := do(r, http.MethodPost, "/api/posts", token, map[string]any{
		"title": "Artikel Router", "category": "Umum", "content": "isi", "is_published": true})
	expect(t, w, 201, "create post")
	expect(t, do(r, http.MethodGet, "/api/posts", token, nil), 200, "list posts")
	expect(t, do(r, http.MethodGet, "/api/posts/stats", token, nil), 200, "stats")
	expect(t, do(r, http.MethodGet, "/api/public/posts/artikel-router", "", nil), 200, "public detail ok")
	expect(t, do(r, http.MethodPost, "/api/posts/ai-generate", token, map[string]string{}), 400, "ai-generate validation")

	// admin
	expect(t, do(r, http.MethodGet, "/api/admin/users", token, nil), 403, "member forbidden")
	admin := login(t, r, "admin", "admin")
	expect(t, do(r, http.MethodGet, "/api/admin/users", admin, nil), 200, "admin users")
	expect(t, do(r, http.MethodGet, "/api/admin/posts", admin, nil), 200, "admin posts")
	expect(t, do(r, http.MethodGet, "/api/admin/users/1/posts", admin, nil), 200, "admin user posts")
}

func TestRouter_CORSAndNotFound(t *testing.T) {
	r, _ := newTestRouter(t)

	w := do(r, http.MethodOptions, "/api/posts", "", nil)
	expect(t, w, 204, "preflight")
	if w.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Fatal("missing CORS header")
	}

	w = do(r, http.MethodGet, "/tidak/ada", "", nil)
	expect(t, w, 404, "not found")
	if !strings.Contains(w.Body.String(), "Endpoint not found") {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}
}

func TestRouter_APIJSONAndSwagger(t *testing.T) {
	r, _ := newTestRouter(t)

	hostOf := func(req *http.Request) (string, string) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("api.json status %d", w.Code)
		}
		var spec struct {
			Host    string   `json:"host"`
			Schemes []string `json:"schemes"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &spec); err != nil {
			t.Fatalf("invalid json: %v", err)
		}
		return spec.Host, spec.Schemes[0]
	}

	req := httptest.NewRequest(http.MethodGet, "/api.json", nil)
	req.Host = "api.example.com"
	if h, s := hostOf(req); h != "api.example.com" || s != "http" {
		t.Fatalf("plain: %s %s", h, s)
	}

	req = httptest.NewRequest(http.MethodGet, "/api.json", nil)
	req.Host = "proxy.example.com"
	req.Header.Set("X-Forwarded-Proto", "https")
	if h, s := hostOf(req); h != "proxy.example.com" || s != "https" {
		t.Fatalf("forwarded: %s %s", h, s)
	}

	req = httptest.NewRequest(http.MethodGet, "/api.json", nil)
	req.Host = "tls.example.com"
	req.TLS = &tls.ConnectionState{}
	if h, s := hostOf(req); h != "tls.example.com" || s != "https" {
		t.Fatalf("tls: %s %s", h, s)
	}

	// Swagger UI and doc.json
	expect(t, do(r, http.MethodGet, "/swagger/index.html", "", nil), 200, "swagger index")
	w := do(r, http.MethodGet, "/swagger/doc.json", "", nil)
	expect(t, w, 200, "swagger doc.json")
	if !strings.Contains(w.Body.String(), "Delcom Blog API") {
		t.Fatal("doc.json missing title")
	}
}

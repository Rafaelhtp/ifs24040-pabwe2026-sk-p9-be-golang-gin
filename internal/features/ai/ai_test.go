package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"app/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const secret = "ai_test_secret"

func init() {
	gin.SetMode(gin.TestMode)
}

type mock struct {
	searchStatus int
	searchBody   string
	chatStatus   int
	chatBody     string
	lastQuery    string
	lastAuth     string
}

func articleChat(content string) string {
	b, _ := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": content}}},
	})
	return string(b)
}

const goodArticle = `{"title":"Judul AI","category":"Teknologi","excerpt":"Ringkas","content":"# Isi"}`

func newMock() (*mock, *httptest.Server) {
	m := &mock{
		searchStatus: 200,
		searchBody:   `{"results":[{"title":"T1","url":"http://a","snippet":"S1"}]}`,
		chatStatus:   200,
		chatBody:     articleChat(goodArticle),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		m.lastQuery = r.URL.Query().Get("q")
		w.WriteHeader(m.searchStatus)
		_, _ = w.Write([]byte(m.searchBody))
	})
	mux.HandleFunc("/v1/chat/completions", func(w http.ResponseWriter, r *http.Request) {
		m.lastAuth = r.Header.Get("Authorization")
		w.WriteHeader(m.chatStatus)
		_, _ = w.Write([]byte(m.chatBody))
	})
	return m, httptest.NewServer(mux)
}

func newRouter(srv *httptest.Server) *gin.Engine {
	svc := NewService(NewSearchClient(srv.URL+"/search"), NewOpenAIClient(srv.URL+"/v1/", "key", "model"))
	r := gin.New()
	NewHandler(svc).RegisterRoutes(r.Group("/api"), middleware.Auth(secret))
	return r
}

func call(r http.Handler, body any) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if s, ok := body.(string); ok {
		buf.WriteString(s)
	} else {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, &middleware.Claims{UserID: 1, Role: "member"}).SignedString([]byte(secret))
	req := httptest.NewRequest(http.MethodPost, "/api/posts/ai-generate", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func expectCode(t *testing.T, w *httptest.ResponseRecorder, code int, label string) {
	t.Helper()
	if w.Code != code {
		t.Fatalf("%s: expected %d, got %d: %s", label, code, w.Code, w.Body.String())
	}
}

func TestGenerate_Success(t *testing.T) {
	m, srv := newMock()
	defer srv.Close()
	r := newRouter(srv)

	// with web search
	w := call(r, GenerateRequest{Topic: "Golang", Category: "Teknologi", IncludeWebSearch: true})
	expectCode(t, w, 200, "success with search")
	var out struct {
		Data GenerateResponse `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Data.Title != "Judul AI" || len(out.Data.SearchSources) != 1 || out.Data.SearchSources[0].Title != "T1" {
		t.Fatalf("unexpected output: %+v", out.Data)
	}
	if m.lastQuery != "Golang" || m.lastAuth != "Bearer key" {
		t.Fatalf("unexpected upstream calls: %q %q", m.lastQuery, m.lastAuth)
	}

	// without web search, default category, fenced JSON, invalid category from AI falls back
	m.chatBody = articleChat("```json\n" + `{"title":"J","category":"Ngawur","excerpt":"e","content":"c"}` + "\n```")
	w = call(r, GenerateRequest{Topic: "Apa saja"})
	expectCode(t, w, 200, "success without search")
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	if out.Data.Category != "Umum" || out.Data.SearchSources == nil || len(out.Data.SearchSources) != 0 {
		t.Fatalf("unexpected fallback: %+v", out.Data)
	}
}

func TestGenerate_Errors(t *testing.T) {
	m, srv := newMock()
	defer srv.Close()
	r := newRouter(srv)

	expectCode(t, call(r, "{bad"), 400, "bad json")
	expectCode(t, call(r, GenerateRequest{}), 400, "missing topic")
	expectCode(t, call(r, GenerateRequest{Topic: "x", Category: "Lain"}), 400, "invalid category")

	// search failures
	m.searchStatus = 500
	expectCode(t, call(r, GenerateRequest{Topic: "x", IncludeWebSearch: true}), 500, "search status")
	m.searchStatus = 200
	m.searchBody = "not json"
	expectCode(t, call(r, GenerateRequest{Topic: "x", IncludeWebSearch: true}), 500, "search invalid json")
	m.searchBody = `{"results":[]}`

	// openai failures
	m.chatStatus = 500
	expectCode(t, call(r, GenerateRequest{Topic: "x"}), 500, "openai status")
	m.chatStatus = 200
	m.chatBody = "not json"
	expectCode(t, call(r, GenerateRequest{Topic: "x"}), 500, "openai invalid envelope")
	m.chatBody = `{"choices":[]}`
	expectCode(t, call(r, GenerateRequest{Topic: "x"}), 500, "openai no choices")
	m.chatBody = articleChat("bukan json")
	expectCode(t, call(r, GenerateRequest{Topic: "x"}), 500, "openai invalid article json")
	m.chatBody = articleChat(`{"title":"","content":""}`)
	expectCode(t, call(r, GenerateRequest{Topic: "x"}), 500, "openai incomplete article")
}

func TestClients_TransportErrors(t *testing.T) {
	_, srv := newMock()
	url := srv.URL
	srv.Close() // connection refused

	if _, err := NewSearchClient(url+"/search").Search(context.Background(), "x"); err == nil {
		t.Fatal("expected search transport error")
	}
	if _, err := NewOpenAIClient(url, "k", "m").Generate(context.Background(), "x", "Umum", nil); err == nil {
		t.Fatal("expected openai transport error")
	}

	// invalid URLs fail at request creation
	bad := "http://invalid host\x7f"
	if _, err := NewSearchClient(bad).Search(context.Background(), "x"); err == nil {
		t.Fatal("expected search request error")
	}
	if _, err := NewOpenAIClient(bad, "k", "m").Generate(context.Background(), "x", "Umum", nil); err == nil {
		t.Fatal("expected openai request error")
	}
}

func TestBuildPrompt(t *testing.T) {
	with := buildPrompt("T", "Umum", []SearchSource{{Title: "A", URL: "u", Snippet: "s"}})
	without := buildPrompt("T", "Umum", nil)
	if !strings.Contains(with, "referensi") || strings.Contains(without, "referensi") {
		t.Fatal("prompt context mismatch")
	}
}

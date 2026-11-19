package post

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"app/internal/config"
	"app/internal/database"
	"app/internal/middleware"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

const testSecret = "post_test_secret"

func init() {
	gin.SetMode(gin.TestMode)
}

type faults struct {
	failAll    bool
	failAt     int
	counter    int
	failCreate bool
	failUpdate bool
	failDelete bool
}

func (f *faults) reset() {
	*f = faults{}
}

func (f *faults) failQueryAt(n int) {
	f.reset()
	f.failAt = n
}

type env struct {
	r      *gin.Engine
	db     *gorm.DB
	f      *faults
	tokens map[string]string
	users  map[string]uint
}

func makeToken(t *testing.T, id uint, role string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, &middleware.Claims{UserID: id, Username: "u", Role: role})
	s, err := tok.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return s
}

func setup(t *testing.T) *env {
	t.Helper()
	db, err := database.ConnectDatabase(&config.Config{DBConnection: "sqlite", DBDatabase: ":memory:"})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	f := &faults{}
	fail := func(d *gorm.DB) { _ = d.AddError(gorm.ErrInvalidDB) }
	_ = db.Callback().Query().Before("gorm:query").Register("fault_query", func(d *gorm.DB) {
		f.counter++
		if f.failAll || (f.failAt > 0 && f.counter == f.failAt) {
			fail(d)
		}
	})
	_ = db.Callback().Create().Before("gorm:create").Register("fault_create", func(d *gorm.DB) {
		if f.failCreate {
			fail(d)
		}
	})
	_ = db.Callback().Update().Before("gorm:update").Register("fault_update", func(d *gorm.DB) {
		if f.failUpdate {
			fail(d)
		}
	})
	_ = db.Callback().Delete().Before("gorm:delete").Register("fault_delete", func(d *gorm.DB) {
		if f.failDelete {
			fail(d)
		}
	})

	e := &env{db: db, f: f, tokens: map[string]string{}, users: map[string]uint{}}
	for _, u := range []struct{ name, role string }{{"u1", "member"}, {"u2", "member"}, {"adm", "admin"}} {
		user := database.User{Username: u.name, Email: u.name + "@x.org", Password: "x", Role: u.role}
		if err := db.Create(&user).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
		e.users[u.name] = user.ID
		e.tokens[u.name] = makeToken(t, user.ID, u.role)
	}

	e.r = gin.New()
	NewHandler(NewService(NewRepository(db))).RegisterRoutes(e.r.Group("/api"), middleware.Auth(testSecret))
	return e
}

func (e *env) do(method, path string, body any, who string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		if s, ok := body.(string); ok {
			buf.WriteString(s)
		} else {
			_ = json.NewEncoder(&buf).Encode(body)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if who != "" {
		req.Header.Set("Authorization", "Bearer "+e.tokens[who])
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

func expect(t *testing.T, w *httptest.ResponseRecorder, code int, label string) {
	t.Helper()
	if w.Code != code {
		t.Fatalf("%s: expected %d, got %d: %s", label, code, w.Code, w.Body.String())
	}
}

type postEnvelope struct {
	Data PostResponse `json:"data"`
}

func TestHelpers(t *testing.T) {
	if CalculateReadingTime("a b c") != 1 {
		t.Fatal("expected min 1")
	}
	if CalculateReadingTime(strings.Repeat("kata ", 400)) != 2 {
		t.Fatal("expected 2 minutes")
	}
	if Slugify("  Halo, Dunia!! ") != "halo-dunia" {
		t.Fatal("slugify failed")
	}
	if Slugify("!!!") != "artikel" {
		t.Fatal("slugify fallback failed")
	}
	if !validCategory("Teknologi") || validCategory("Lain") {
		t.Fatal("validCategory failed")
	}
	p := database.Post{Content: "x"}
	if ToPostResponse(p, false).Author != nil || ToPostResponse(p, true).Author == nil {
		t.Fatal("author flag failed")
	}
}

func TestPostLifecycle(t *testing.T) {
	e := setup(t)

	// categories
	w := e.do(http.MethodGet, "/api/public/categories", nil, "")
	expect(t, w, 200, "categories")

	// create validations
	expect(t, e.do(http.MethodPost, "/api/posts", "{bad", "u1"), 400, "bad json")
	expect(t, e.do(http.MethodPost, "/api/posts", PostRequest{Title: "x"}, "u1"), 400, "missing fields")
	expect(t, e.do(http.MethodPost, "/api/posts", PostRequest{Title: "x", Category: "Lain", Content: "c"}, "u1"), 400, "invalid category")

	// create draft, duplicate title and published
	w = e.do(http.MethodPost, "/api/posts", PostRequest{Title: "Belajar Go", Category: "Teknologi", Content: "isi satu", Excerpt: "ringkas"}, "u1")
	expect(t, w, 201, "create draft")
	var draft postEnvelope
	_ = json.Unmarshal(w.Body.Bytes(), &draft)
	if draft.Data.Slug != "belajar-go" || draft.Data.IsPublished {
		t.Fatalf("unexpected draft: %+v", draft.Data)
	}

	w = e.do(http.MethodPost, "/api/posts", PostRequest{Title: "Belajar Go", Category: "Teknologi", Content: "isi dua"}, "u1")
	expect(t, w, 201, "create duplicate title")
	var dup postEnvelope
	_ = json.Unmarshal(w.Body.Bytes(), &dup)
	if dup.Data.Slug != "belajar-go-2" {
		t.Fatalf("expected belajar-go-2, got %s", dup.Data.Slug)
	}

	w = e.do(http.MethodPost, "/api/posts", PostRequest{Title: "Post Terbit", Category: "Bisnis", Content: strings.Repeat("kata ", 400), IsPublished: true}, "u1")
	expect(t, w, 201, "create published")
	var pub postEnvelope
	_ = json.Unmarshal(w.Body.Bytes(), &pub)
	if pub.Data.PublishedAt == nil || pub.Data.ReadingTime != 2 {
		t.Fatalf("unexpected published: %+v", pub.Data)
	}

	// list / get mine
	expect(t, e.do(http.MethodGet, "/api/posts", nil, "u1"), 200, "list mine")
	id := strconv.Itoa(int(draft.Data.ID))
	expect(t, e.do(http.MethodGet, "/api/posts/"+id, nil, "u1"), 200, "get mine")
	expect(t, e.do(http.MethodGet, "/api/posts/"+id, nil, "u2"), 403, "get other's")
	expect(t, e.do(http.MethodGet, "/api/posts/abc", nil, "u1"), 400, "get bad id")
	expect(t, e.do(http.MethodGet, "/api/posts/9999", nil, "u1"), 404, "get not found")

	// default category + draft => published_at null
	w = e.do(http.MethodPost, "/api/posts", PostRequest{Title: "Tanpa Kategori", Content: "isi"}, "u1")
	expect(t, w, 201, "create default category")
	var def postEnvelope
	_ = json.Unmarshal(w.Body.Bytes(), &def)
	if def.Data.Category != "Umum" || def.Data.PublishedAt != nil || def.Data.IsPublished {
		t.Fatalf("unexpected default post: %+v", def.Data)
	}
	expect(t, e.do(http.MethodDelete, "/api/posts/"+strconv.Itoa(int(def.Data.ID)), nil, "u1"), 200, "delete default post")

	// update (partial)
	s := func(v string) *string { return &v }
	b := func(v bool) *bool { return &v }
	expect(t, e.do(http.MethodPut, "/api/posts/abc", UpdatePostRequest{}, "u1"), 400, "update bad id")
	expect(t, e.do(http.MethodPut, "/api/posts/"+id, "{bad", "u1"), 400, "update bad json")
	expect(t, e.do(http.MethodPut, "/api/posts/9999", UpdatePostRequest{}, "u1"), 404, "update not found")
	expect(t, e.do(http.MethodPut, "/api/posts/"+id, UpdatePostRequest{Title: s("x")}, "u2"), 403, "update other's")
	expect(t, e.do(http.MethodPut, "/api/posts/"+id, UpdatePostRequest{Category: s("Lain")}, "u1"), 400, "update bad category")
	expect(t, e.do(http.MethodPut, "/api/posts/"+id, UpdatePostRequest{Title: s("  ")}, "u1"), 400, "update empty title")
	expect(t, e.do(http.MethodPut, "/api/posts/"+id, UpdatePostRequest{Content: s("")}, "u1"), 400, "update empty content")
	expect(t, e.do(http.MethodPut, "/api/posts/"+id, UpdatePostRequest{}, "u1"), 200, "update no-op")
	expect(t, e.do(http.MethodPut, "/api/posts/"+id, UpdatePostRequest{Title: s("Belajar Go"), Content: s("baru")}, "u1"), 200, "update same title + content")
	expect(t, e.do(http.MethodPut, "/api/posts/"+id, UpdatePostRequest{Title: s("Judul Baru"), IsPublished: b(true)}, "u1"), 200, "update new title + publish")
	expect(t, e.do(http.MethodPut, "/api/posts/"+id, UpdatePostRequest{IsPublished: b(true)}, "u1"), 200, "update already published")
	// rename into colliding slug (belajar-go-2 owned by other post -> loop)
	collide := UpdatePostRequest{Title: s("Belajar Go 2"), Category: s("Umum"), Excerpt: s("e"), CoverImagePath: s("/c.png"), IsPublished: b(false)}
	w = e.do(http.MethodPatch, "/api/posts/"+id, collide, "u1")
	expect(t, w, 200, "patch collide + unpublish")
	var upd postEnvelope
	_ = json.Unmarshal(w.Body.Bytes(), &upd)
	if upd.Data.Slug != "belajar-go-2-2" || upd.Data.PublishedAt != nil || upd.Data.Category != "Umum" {
		t.Fatalf("unexpected updated post: %+v", upd.Data)
	}

	// toggle
	expect(t, e.do(http.MethodPatch, "/api/posts/abc/toggle-publish", nil, "u1"), 400, "toggle bad id")
	expect(t, e.do(http.MethodPatch, "/api/posts/9999/toggle-publish", nil, "u1"), 404, "toggle not found")
	expect(t, e.do(http.MethodPatch, "/api/posts/"+id+"/toggle-publish", nil, "u2"), 403, "toggle other's")
	expect(t, e.do(http.MethodPatch, "/api/posts/"+id+"/toggle-publish", nil, "u1"), 200, "toggle on")
	expect(t, e.do(http.MethodPatch, "/api/posts/"+id+"/toggle-publish", nil, "u1"), 200, "toggle off")

	// stats
	w = e.do(http.MethodGet, "/api/posts/stats", nil, "u1")
	expect(t, w, 200, "stats")
	var st struct {
		Data StatsResponse `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &st)
	if st.Data.TotalArticles != 3 || st.Data.TotalSelesai != 1 || st.Data.BelumSelesai != 2 {
		t.Fatalf("unexpected stats: %+v", st.Data)
	}

	// public
	expect(t, e.do(http.MethodGet, "/api/public/posts", nil, ""), 200, "public list")
	w = e.do(http.MethodGet, "/api/public/posts?q=kata&category=Bisnis", nil, "")
	expect(t, w, 200, "public filtered")
	if !strings.Contains(w.Body.String(), "post-terbit") {
		t.Fatalf("filter result missing post: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "user_id") || !strings.Contains(w.Body.String(), `"author"`) {
		t.Fatalf("public list must hide user_id and include author: %s", w.Body.String())
	}
	w = e.do(http.MethodGet, "/api/public/posts/post-terbit", nil, "")
	expect(t, w, 200, "public detail")
	if strings.Contains(w.Body.String(), "user_id") {
		t.Fatalf("public detail must hide user_id: %s", w.Body.String())
	}
	expect(t, e.do(http.MethodGet, "/api/public/posts/"+upd.Data.Slug, nil, ""), 404, "public draft detail")

	// admin
	expect(t, e.do(http.MethodGet, "/api/admin/users", nil, "u1"), 403, "member forbidden")
	expect(t, e.do(http.MethodGet, "/api/admin/users", nil, "adm"), 200, "admin users")
	expect(t, e.do(http.MethodGet, "/api/admin/posts", nil, "adm"), 200, "admin posts")
	uid := strconv.Itoa(int(e.users["u1"]))
	expect(t, e.do(http.MethodGet, "/api/admin/users/"+uid+"/posts", nil, "adm"), 200, "admin user posts")
	expect(t, e.do(http.MethodGet, "/api/admin/users/abc/posts", nil, "adm"), 400, "admin bad id")
	w = e.do(http.MethodGet, "/api/admin/users/9999/posts", nil, "adm")
	expect(t, w, 200, "admin unknown user")
	if !strings.Contains(w.Body.String(), `"data":[]`) {
		t.Fatalf("expected empty data array: %s", w.Body.String())
	}

	// delete
	expect(t, e.do(http.MethodDelete, "/api/posts/abc", nil, "u1"), 400, "delete bad id")
	expect(t, e.do(http.MethodDelete, "/api/posts/9999", nil, "u1"), 404, "delete not found")
	expect(t, e.do(http.MethodDelete, "/api/posts/"+id, nil, "u2"), 403, "delete other's")
	expect(t, e.do(http.MethodDelete, "/api/posts/"+id, nil, "u1"), 200, "delete")
}

func TestPostFaults(t *testing.T) {
	e := setup(t)
	w := e.do(http.MethodPost, "/api/posts", PostRequest{Title: "Satu", Category: "Umum", Content: "c"}, "u1")
	expect(t, w, 201, "seed post")
	var seeded postEnvelope
	_ = json.Unmarshal(w.Body.Bytes(), &seeded)
	id := strconv.Itoa(int(seeded.Data.ID))
	uid := strconv.Itoa(int(e.users["u1"]))
	valid := PostRequest{Title: "Satu", Category: "Umum", Content: "c"}

	// all queries fail
	e.f.reset()
	e.f.failAll = true
	expect(t, e.do(http.MethodGet, "/api/public/posts", nil, ""), 500, "public list")
	expect(t, e.do(http.MethodGet, "/api/posts", nil, "u1"), 500, "list mine")
	expect(t, e.do(http.MethodPost, "/api/posts", valid, "u1"), 500, "create slug check")
	expect(t, e.do(http.MethodGet, "/api/posts/stats", nil, "u1"), 500, "stats first count")
	expect(t, e.do(http.MethodGet, "/api/admin/users", nil, "adm"), 500, "admin users list")
	expect(t, e.do(http.MethodGet, "/api/admin/posts", nil, "adm"), 500, "admin posts")
	expect(t, e.do(http.MethodGet, "/api/admin/users/"+uid+"/posts", nil, "adm"), 500, "admin user posts")

	// second / third query fails
	e.f.failQueryAt(2)
	expect(t, e.do(http.MethodGet, "/api/posts/stats", nil, "u1"), 500, "stats second count")
	e.f.failQueryAt(2)
	expect(t, e.do(http.MethodGet, "/api/admin/users", nil, "adm"), 500, "admin users first count")
	e.f.failQueryAt(3)
	expect(t, e.do(http.MethodGet, "/api/admin/users", nil, "adm"), 500, "admin users second count")
	e.f.failQueryAt(2)
	newTitle := "Judul Lain"
	expect(t, e.do(http.MethodPut, "/api/posts/"+id, UpdatePostRequest{Title: &newTitle}, "u1"), 500, "update slug check")

	// write failures
	e.f.reset()
	e.f.failCreate = true
	expect(t, e.do(http.MethodPost, "/api/posts", valid, "u1"), 500, "create fail")
	e.f.reset()
	e.f.failUpdate = true
	expect(t, e.do(http.MethodPut, "/api/posts/"+id, UpdatePostRequest{}, "u1"), 500, "update fail")
	expect(t, e.do(http.MethodPatch, "/api/posts/"+id+"/toggle-publish", nil, "u1"), 500, "toggle fail")
	e.f.reset()
	e.f.failDelete = true
	expect(t, e.do(http.MethodDelete, "/api/posts/"+id, nil, "u1"), 500, "delete fail")
}

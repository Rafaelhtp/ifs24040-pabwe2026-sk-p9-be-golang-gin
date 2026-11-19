package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"app/internal/config"
	"app/internal/database"
	"app/internal/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupAuthTest(t *testing.T) (*gin.Engine, *gorm.DB, *Service, string) {
	t.Helper()
	cfg := &config.Config{
		DBConnection: "sqlite",
		DBDatabase:   ":memory:",
	}
	db, err := database.ConnectDatabase(cfg)
	if err != nil {
		t.Fatalf("failed to connect db: %v", err)
	}
	if err := database.AutoMigrate(db); err != nil {
		t.Fatalf("failed to migrate db: %v", err)
	}

	secret := "test_auth_secret"
	repo := NewRepository(db)
	svc := NewService(repo, secret)
	handler := NewHandler(svc)

	r := gin.New()
	api := r.Group("/api")
	handler.RegisterRoutes(api, middleware.Auth(secret))

	return r, db, svc, secret
}

func doJSONRequest(r http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		if str, ok := body.(string); ok {
			buf.WriteString(str)
		} else {
			_ = json.NewEncoder(&buf).Encode(body)
		}
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

func TestAuth_FullLifecycle(t *testing.T) {
	r, _, _, _ := setupAuthTest(t)

	// 1. Register validation failure
	wBadReg := doJSONRequest(r, http.MethodPost, "/api/auth/register", "{invalid", "")
	if wBadReg.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad register json, got %d", wBadReg.Code)
	}

	// 2. Register success
	regPayload := RegisterRequest{
		Username: "penulis1",
		Email:    "penulis1@delcom.org",
		Password: "password123",
	}
	wReg := doJSONRequest(r, http.MethodPost, "/api/auth/register", regPayload, "")
	if wReg.Code != http.StatusCreated {
		t.Fatalf("expected 201 on register, got %d: %s", wReg.Code, wReg.Body.String())
	}
	var regResp struct {
		Data struct {
			Token string         `json:"token"`
			User  map[string]any `json:"user"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wReg.Body.Bytes(), &regResp)
	if regResp.Data.Token == "" || regResp.Data.User["username"] != "penulis1" || regResp.Data.User["role"] != "member" {
		t.Fatalf("register must return token and user: %s", wReg.Body.String())
	}
	if _, hasPass := regResp.Data.User["password"]; hasPass {
		t.Fatalf("user must not expose password")
	}

	// 2a. Register with role admin and unknown role fallback
	wAdminReg := doJSONRequest(r, http.MethodPost, "/api/auth/register", RegisterRequest{
		Username: "admin_reg",
		Email:    "admin_reg@delcom.org",
		Password: "password123",
		Role:     "admin",
	}, "")
	if wAdminReg.Code != http.StatusCreated {
		t.Fatalf("expected 201 on admin register, got %d: %s", wAdminReg.Code, wAdminReg.Body.String())
	}
	var adminRegResp struct {
		Data AuthResponse `json:"data"`
	}
	_ = json.Unmarshal(wAdminReg.Body.Bytes(), &adminRegResp)
	if adminRegResp.Data.User.Role != database.RoleAdmin {
		t.Fatalf("expected role admin, got %q", adminRegResp.Data.User.Role)
	}

	wUnknownRoleReg := doJSONRequest(r, http.MethodPost, "/api/auth/register", RegisterRequest{
		Username: "super_reg",
		Email:    "super_reg@delcom.org",
		Password: "password123",
		Role:     "super",
	}, "")
	if wUnknownRoleReg.Code != http.StatusCreated {
		t.Fatalf("expected 201 on unknown role register, got %d: %s", wUnknownRoleReg.Code, wUnknownRoleReg.Body.String())
	}
	var unknownRoleResp struct {
		Data AuthResponse `json:"data"`
	}
	_ = json.Unmarshal(wUnknownRoleReg.Body.Bytes(), &unknownRoleResp)
	if unknownRoleResp.Data.User.Role != database.RoleMember {
		t.Fatalf("expected role member fallback, got %q", unknownRoleResp.Data.User.Role)
	}

	// 2b. Register validation: username kosong, email kosong/invalid, password pendek
	for name, p := range map[string]RegisterRequest{
		"empty username": {Email: "a@delcom.org", Password: "password123"},
		"empty email":    {Username: "abc", Password: "password123"},
		"invalid email":  {Username: "abc", Email: "bukan-email", Password: "password123"},
		"short password": {Username: "abc", Email: "a@delcom.org", Password: "12345"},
	} {
		if w := doJSONRequest(r, http.MethodPost, "/api/auth/register", p, ""); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", name, w.Code)
		}
	}

	// 3. Register duplicate -> 400
	wDup := doJSONRequest(r, http.MethodPost, "/api/auth/register", regPayload, "")
	if wDup.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on duplicate register, got %d", wDup.Code)
	}

	// 4. Register with >72 byte password -> 500 Internal Server Error
	wLongPass := doJSONRequest(r, http.MethodPost, "/api/auth/register", RegisterRequest{
		Username: "penulis_long",
		Email:    "long@delcom.org",
		Password: strings.Repeat("p", 73),
	}, "")
	if wLongPass.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on >72 byte password, got %d", wLongPass.Code)
	}

	// 5. Login validation failure
	wBadLogin := doJSONRequest(r, http.MethodPost, "/api/auth/login", "{invalid", "")
	if wBadLogin.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on bad login json, got %d", wBadLogin.Code)
	}

	// 6. Login user not found
	wNoUser := doJSONRequest(r, http.MethodPost, "/api/auth/login", LoginRequest{
		Username: "tidakada",
		Password: "password123",
	}, "")
	if wNoUser.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on non-existent user, got %d", wNoUser.Code)
	}

	// 7. Login wrong password
	wWrongPass := doJSONRequest(r, http.MethodPost, "/api/auth/login", LoginRequest{
		Username: "penulis1",
		Password: "salahpassword",
	}, "")
	if wWrongPass.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on wrong password, got %d", wWrongPass.Code)
	}

	// 8. Login success
	wLogin := doJSONRequest(r, http.MethodPost, "/api/auth/login", LoginRequest{
		Username: "penulis1",
		Password: "password123",
	}, "")
	if wLogin.Code != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d: %s", wLogin.Code, wLogin.Body.String())
	}

	var loginResp struct {
		Data AuthResponse `json:"data"`
	}
	_ = json.Unmarshal(wLogin.Body.Bytes(), &loginResp)
	token := loginResp.Data.Token
	if token == "" {
		t.Fatalf("expected non-empty JWT token")
	}

	// 9. Get Profile success
	wProf := doJSONRequest(r, http.MethodGet, "/api/auth/profile", nil, token)
	if wProf.Code != http.StatusOK {
		t.Fatalf("expected 200 on get profile, got %d", wProf.Code)
	}

	// 10. Update Profile validation error
	wBadUpd := doJSONRequest(r, http.MethodPut, "/api/auth/profile", "{invalid", token)
	if wBadUpd.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on invalid update json, got %d", wBadUpd.Code)
	}

	// 11. Update Profile success (with new password)
	wUpd := doJSONRequest(r, http.MethodPut, "/api/auth/profile", UpdateProfileRequest{
		Username: "penulis1_updated",
		Email:    "penulis1_new@delcom.org",
		Password: "newpassword123",
	}, token)
	if wUpd.Code != http.StatusOK {
		t.Fatalf("expected 200 on update profile, got %d: %s", wUpd.Code, wUpd.Body.String())
	}

	// 11b. Partial update: hanya email, lalu email + password
	wPartial := doJSONRequest(r, http.MethodPut, "/api/auth/profile", map[string]string{"email": "only_email@delcom.org"}, token)
	if wPartial.Code != http.StatusOK || !strings.Contains(wPartial.Body.String(), "only_email@delcom.org") ||
		!strings.Contains(wPartial.Body.String(), "penulis1_updated") {
		t.Fatalf("partial email update failed: %d %s", wPartial.Code, wPartial.Body.String())
	}
	wPartial = doJSONRequest(r, http.MethodPut, "/api/auth/profile", map[string]string{"email": "e2@delcom.org", "password": "another123"}, token)
	if wPartial.Code != http.StatusOK {
		t.Fatalf("email+password update failed: %d %s", wPartial.Code, wPartial.Body.String())
	}
	wRelogin := doJSONRequest(r, http.MethodPost, "/api/auth/login", LoginRequest{Username: "penulis1_updated", Password: "another123"}, "")
	if wRelogin.Code != http.StatusOK {
		t.Fatalf("login with new password failed: %d", wRelogin.Code)
	}
	wShortPass := doJSONRequest(r, http.MethodPut, "/api/auth/profile", map[string]string{"password": "123"}, token)
	if wShortPass.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on short password update, got %d", wShortPass.Code)
	}

	// 12. Update Profile with >72 byte password -> 500
	wUpdLong := doJSONRequest(r, http.MethodPut, "/api/auth/profile", UpdateProfileRequest{
		Username: "penulis1_updated",
		Email:    "e2@delcom.org",
		Password: strings.Repeat("x", 73),
	}, token)
	if wUpdLong.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on long password update, got %d", wUpdLong.Code)
	}

	// 13. Register second user to test conflict on UpdateProfile
	_ = doJSONRequest(r, http.MethodPost, "/api/auth/register", RegisterRequest{
		Username: "penulis2",
		Email:    "penulis2@delcom.org",
		Password: "password123",
	}, "")

	wUpdConflict := doJSONRequest(r, http.MethodPut, "/api/auth/profile", UpdateProfileRequest{
		Username: "penulis2",
		Email:    "penulis2@delcom.org",
	}, token)
	if wUpdConflict.Code != http.StatusConflict {
		t.Fatalf("expected 409 on update profile conflict, got %d", wUpdConflict.Code)
	}

	// 14. Logout
	wLogout := doJSONRequest(r, http.MethodPost, "/api/auth/logout", nil, token)
	if wLogout.Code != http.StatusOK {
		t.Fatalf("expected 200 on logout, got %d", wLogout.Code)
	}
}

func TestAuth_NotFoundAndDBErrors(t *testing.T) {
	r, db, svc, _ := setupAuthTest(t)

	// Register and login to get a valid token for user ID 1
	_ = doJSONRequest(r, http.MethodPost, "/api/auth/register", RegisterRequest{
		Username: "tempuser",
		Email:    "temp@delcom.org",
		Password: "password123",
	}, "")
	wLogin := doJSONRequest(r, http.MethodPost, "/api/auth/login", LoginRequest{
		Username: "tempuser",
		Password: "password123",
	}, "")
	var loginResp struct {
		Data AuthResponse `json:"data"`
	}
	_ = json.Unmarshal(wLogin.Body.Bytes(), &loginResp)
	token := loginResp.Data.Token

	// Delete user from DB so token refers to non-existent user ID
	db.Exec("DELETE FROM users")

	wProf := doJSONRequest(r, http.MethodGet, "/api/auth/profile", nil, token)
	if wProf.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when user deleted, got %d", wProf.Code)
	}

	wUpd := doJSONRequest(r, http.MethodPut, "/api/auth/profile", UpdateProfileRequest{
		Username: "newname",
		Email:    "new@delcom.org",
	}, token)
	if wUpd.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on update when user deleted, got %d", wUpd.Code)
	}

	// Test DB errors via hooks / dropped table
	// 1. Create failure via callback
	_ = db.Callback().Create().Before("gorm:create").Register("force_create_err", func(d *gorm.DB) {
		_ = d.AddError(gorm.ErrInvalidDB)
	})
	_, err := svc.Register(RegisterRequest{
		Username: "failcreate",
		Email:    "failcreate@delcom.org",
		Password: "password123",
	})
	if err == nil {
		t.Fatalf("expected error on create failure")
	}
	_ = db.Callback().Create().Remove("force_create_err")

	// Create a valid user again for update failure test
	u := database.User{
		Username: "upduser",
		Email:    "upd@delcom.org",
		Password: "hashed",
		Role:     database.RoleMember,
	}
	db.Create(&u)

	// 2. Update failure via callback
	_ = db.Callback().Update().Before("gorm:update").Register("force_update_err", func(d *gorm.DB) {
		_ = d.AddError(gorm.ErrInvalidDB)
	})
	_, err = svc.UpdateProfile(u.ID, UpdateProfileRequest{
		Username: "upduser2",
		Email:    "upd2@delcom.org",
	})
	if err == nil {
		t.Fatalf("expected error on update failure")
	}
	_ = db.Callback().Update().Remove("force_update_err")

	// 3. ExistsByUsernameOrEmail error on UpdateProfile by failing query callback after First
	queryCount := 0
	_ = db.Callback().Query().Before("gorm:query").Register("force_second_query_err", func(d *gorm.DB) {
		queryCount++
		if queryCount == 2 {
			_ = d.AddError(gorm.ErrInvalidDB)
		}
	})
	_, err = svc.UpdateProfile(u.ID, UpdateProfileRequest{
		Username: "upduser3",
		Email:    "upd3@delcom.org",
	})
	if err == nil {
		t.Fatalf("expected error on second query failure in UpdateProfile")
	}
	_ = db.Callback().Query().Remove("force_second_query_err")

	// 4. Close DB to trigger ExistsByUsernameOrEmail error on Register
	sqlDB, _ := db.DB()
	_ = sqlDB.Close()
	_, err = svc.Register(RegisterRequest{
		Username: "closeddb",
		Email:    "closed@delcom.org",
		Password: "password123",
	})
	if err == nil {
		t.Fatalf("expected error when db closed")
	}
}

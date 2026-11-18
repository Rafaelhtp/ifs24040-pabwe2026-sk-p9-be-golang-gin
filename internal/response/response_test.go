package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestSuccess_WithDataAndWithoutData(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Success(c, http.StatusOK, "Berhasil mengambil data", gin.H{"id": 1})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body["status"] != "success" || body["message"] != "Berhasil mengambil data" || body["data"] == nil {
		t.Fatalf("unexpected body: %+v", body)
	}

	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	Success(c2, http.StatusOK, "Berhasil tanpa data")

	var body2 map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &body2); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if _, hasData := body2["data"]; hasData {
		t.Fatalf("expected data to be omitted when nil")
	}
}

func TestFail_And_Error(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	Fail(c, http.StatusBadRequest, "Input tidak valid", gin.H{"field": "title"})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var failBody map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &failBody); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if failBody["status"] != "fail" || failBody["message"] != "Input tidak valid" || failBody["data"] == nil {
		t.Fatalf("unexpected failBody: %+v", failBody)
	}

	wErr := httptest.NewRecorder()
	cErr, _ := gin.CreateTestContext(wErr)

	Error(cErr, http.StatusInternalServerError, "Terjadi kesalahan server")

	if wErr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", wErr.Code)
	}
	var errBody map[string]any
	if err := json.Unmarshal(wErr.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if errBody["status"] != "error" || errBody["message"] != "Terjadi kesalahan server" {
		t.Fatalf("unexpected errBody: %+v", errBody)
	}
	if _, hasData := errBody["data"]; hasData {
		t.Fatalf("expected data to be omitted")
	}
}

func TestAbortFail_And_AbortError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	AbortFail(c, http.StatusUnauthorized, "Token tidak valid")

	if !c.IsAborted() || w.Code != http.StatusUnauthorized {
		t.Fatalf("expected aborted 401, got aborted=%v code=%d", c.IsAborted(), w.Code)
	}
	var failBody Response
	if err := json.Unmarshal(w.Body.Bytes(), &failBody); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if failBody.Status != "fail" || failBody.Message != "Token tidak valid" {
		t.Fatalf("unexpected failBody: %+v", failBody)
	}

	wErr := httptest.NewRecorder()
	cErr, _ := gin.CreateTestContext(wErr)

	AbortError(cErr, http.StatusInternalServerError, "Fatal error")

	if !cErr.IsAborted() || wErr.Code != http.StatusInternalServerError {
		t.Fatalf("expected aborted 500, got aborted=%v code=%d", cErr.IsAborted(), wErr.Code)
	}
	var errBody Response
	if err := json.Unmarshal(wErr.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if errBody.Status != "error" || errBody.Message != "Fatal error" {
		t.Fatalf("unexpected errBody: %+v", errBody)
	}
}

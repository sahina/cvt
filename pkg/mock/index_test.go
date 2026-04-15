package mock

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sahina/cvt/pkg/cvt"
)

func TestIndexPage_WithEndpoints(t *testing.T) {
	v := cvt.NewValidator()
	if err := v.RegisterSchema("test", []byte(testSchema)); err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	handler := IndexHandler(v, []string{"test"})
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "CVT Mock Server") {
		t.Error("expected page title")
	}
	if !strings.Contains(body, "/users") {
		t.Error("expected /users endpoint listed")
	}
	if !strings.Contains(body, "GET") {
		t.Error("expected GET method listed")
	}
}

func TestIndexPage_NoEndpoints(t *testing.T) {
	v := cvt.NewValidator()
	emptySchema := `{"openapi":"3.0.0","info":{"title":"Empty","version":"1.0.0"},"paths":{}}`
	if err := v.RegisterSchema("empty", []byte(emptySchema)); err != nil {
		t.Fatalf("RegisterSchema failed: %v", err)
	}

	handler := IndexHandler(v, []string{"empty"})
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "No endpoints") {
		t.Error("expected 'No endpoints' message for empty schema")
	}
}

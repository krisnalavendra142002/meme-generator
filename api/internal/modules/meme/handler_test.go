package meme

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandler_Delete_RequiresAuth(t *testing.T) {
	h := NewHandler(nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/memes/meme-123", nil)
	rec := httptest.NewRecorder()
	h.Delete(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 when unauthenticated, got %d", rec.Code)
	}
}

func TestHandler_Get_RequiresAuth(t *testing.T) {
	h := NewHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/memes/meme-123", nil)
	rec := httptest.NewRecorder()
	h.Get(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 when unauthenticated, got %d", rec.Code)
	}
}

func TestRegisterRoutes_IncludesGetAndOwnershipProtectedRoutes(t *testing.T) {
	mux := http.NewServeMux()
	h := NewHandler(nil)
	authCalled := false
	dummyAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authCalled = true
			w.WriteHeader(http.StatusOK)
		})
	}
	RegisterRoutes(mux, h, dummyAuth)

	// Test GET route pattern registration
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/memes/123", nil)
	recGet := httptest.NewRecorder()
	authCalled = false
	mux.ServeHTTP(recGet, reqGet)
	if !authCalled {
		t.Errorf("expected GET /api/v1/memes/{id} to be wrapped with auth middleware")
	}

	// Test DELETE route pattern registration
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/memes/123", nil)
	recDel := httptest.NewRecorder()
	authCalled = false
	mux.ServeHTTP(recDel, reqDel)
	if !authCalled {
		t.Errorf("expected DELETE /api/v1/memes/{id} to be wrapped with auth middleware")
	}
}

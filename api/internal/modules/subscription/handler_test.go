package subscription

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nyxsr/meme-portal/api/internal/middleware"
)

func TestHandler_Checkout_RequiresIdempotencyKey(t *testing.T) {
	h := NewHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/subscription/checkout", bytes.NewBufferString(`{"planCode":"free"}`))
	req.Header.Set("Content-Type", "application/json")
	// inject user ID in context
	ctx := middleware.ContextWithUserID(context.Background(), "user-123")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	h.Checkout(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 when Idempotency-Key is missing, got %d", rec.Code)
	}
}

func TestHandler_Status_SetsNoStoreHeaders(t *testing.T) {
	h := NewHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscription/status", nil)
	rec := httptest.NewRecorder()
	h.Status(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc != "no-store, no-cache, must-revalidate" {
		t.Fatalf("expected Cache-Control 'no-store, no-cache, must-revalidate', got '%s'", cc)
	}
	pragma := rec.Header().Get("Pragma")
	if pragma != "no-cache" {
		t.Fatalf("expected Pragma 'no-cache', got '%s'", pragma)
	}
}

func TestRegisterRoutes_Status_DoesNotHavePublicCache(t *testing.T) {
	mux := http.NewServeMux()
	h := NewHandler(nil)
	dummyAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
	RegisterRoutes(mux, h, dummyAuth)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/subscription/status", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	cc := rec.Header().Get("Cache-Control")
	if cc == "public, max-age=86400" {
		t.Fatalf("Cache-Control must not be 'public, max-age=86400', got '%s'", cc)
	}
	if cc != "no-store, no-cache, must-revalidate" {
		t.Fatalf("expected Cache-Control 'no-store, no-cache, must-revalidate', got '%s'", cc)
	}
}

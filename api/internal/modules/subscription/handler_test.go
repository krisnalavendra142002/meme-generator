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

package meme

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nyxsr/meme-portal/api/internal/config"
	"github.com/nyxsr/meme-portal/api/internal/middleware"
	"github.com/nyxsr/meme-portal/api/internal/response"
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

type mockGate struct{}

func (mockGate) CanCreate(ctx context.Context, userID string) error {
	return nil
}

func TestHandler_PrepareUpload_RejectsInvalidExtensionsAndSizes(t *testing.T) {
	svc := NewService(nil, nil, config.StorageConfig{}, mockGate{})
	h := NewHandler(svc)

	tests := []struct {
		name     string
		filename string
		mime     string
		size     int64
	}{
		{
			name:     "rejects GTA V zip installer",
			filename: "GTA.V.Full.Installer.zip",
			mime:     "application/zip",
			size:     90 * 1024 * 1024 * 1024,
		},
		{
			name:     "rejects executable",
			filename: "setup.exe",
			mime:     "application/x-msdownload",
			size:     1024,
		},
		{
			name:     "rejects html payload",
			filename: "index.html",
			mime:     "text/html",
			size:     500,
		},
		{
			name:     "rejects oversized png file",
			filename: "giant.png",
			mime:     "image/png",
			size:     15 * 1024 * 1024, // 15MB > 10MB limit
		},
		{
			name:     "rejects negative size file",
			filename: "negative.png",
			mime:     "image/png",
			size:     -500,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{
				"title":       "Test",
				"filename":    tc.filename,
				"contentType": tc.mime,
				"size":        tc.size,
			})
			req := httptest.NewRequest(http.MethodPost, "/api/v1/memes/upload", bytes.NewReader(body))
			ctx := middleware.ContextWithUserID(context.Background(), "user-123")
			req = req.WithContext(ctx)

			rec := httptest.NewRecorder()
			h.PrepareUpload(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400 for %s, got %d. Body: %s", tc.name, rec.Code, rec.Body.String())
			}

			var resp struct {
				Success bool   `json:"success"`
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			_ = json.Unmarshal(rec.Body.Bytes(), &resp)
			if resp.Code != response.CodeInvalidImage {
				t.Fatalf("expected code %d, got %d", response.CodeInvalidImage, resp.Code)
			}
		})
	}
}

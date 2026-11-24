package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/nyxsr/meme-portal/api/internal/platform/cookie"
)

func TestClearAuthCookies(t *testing.T) {
	w := httptest.NewRecorder()
	clearAuthCookies(w)

	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}

	foundAccess := false
	foundRefresh := false

	for _, c := range cookies {
		if c.Name == cookie.AccessTokenName {
			foundAccess = true
		}
		if c.Name == cookie.RefreshTokenName {
			foundRefresh = true
		}
		if c.Path != "/" {
			t.Errorf("expected cookie %s to have Path '/', got %q", c.Name, c.Path)
		}
		if c.MaxAge != -1 {
			t.Errorf("expected cookie %s to have MaxAge -1, got %d", c.Name, c.MaxAge)
		}
		if c.Value != "" {
			t.Errorf("expected cookie %s to have empty value, got %q", c.Name, c.Value)
		}
	}

	if !foundAccess || !foundRefresh {
		t.Errorf("missing expected auth cookies in clearAuthCookies response")
	}
}

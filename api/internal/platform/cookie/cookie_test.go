package cookie_test

import (
	"net/http/httptest"
	"testing"

	"github.com/nyxsr/meme-portal/api/internal/platform/cookie"
)

func TestSet(t *testing.T) {
	w := httptest.NewRecorder()
	cookie.Set(w, cookie.AccessTokenName, "token-123", 900, false)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	c := cookies[0]
	if c.Name != cookie.AccessTokenName {
		t.Errorf("expected name %s, got %s", cookie.AccessTokenName, c.Name)
	}
	if c.Value != "token-123" {
		t.Errorf("expected value 'token-123', got %s", c.Value)
	}
	if c.Path != "/" {
		t.Errorf("expected path '/', got %s", c.Path)
	}
	if c.MaxAge != 900 {
		t.Errorf("expected max-age 900, got %d", c.MaxAge)
	}
	if !c.HttpOnly {
		t.Errorf("expected HttpOnly true, got false")
	}
}

func TestClear(t *testing.T) {
	w := httptest.NewRecorder()
	cookie.Clear(w, cookie.AccessTokenName)
	cookie.Clear(w, cookie.RefreshTokenName)

	cookies := w.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}

	for _, c := range cookies {
		if c.Path != "/" {
			t.Errorf("expected cookie %s to have path '/', got %s", c.Name, c.Path)
		}
		if c.MaxAge != -1 {
			t.Errorf("expected cookie %s to have max-age -1, got %d", c.Name, c.MaxAge)
		}
		if c.Value != "" {
			t.Errorf("expected cookie %s to have empty value, got %s", c.Name, c.Value)
		}
		if !c.HttpOnly {
			t.Errorf("expected cookie %s to have HttpOnly true, got false", c.Name)
		}
	}
}

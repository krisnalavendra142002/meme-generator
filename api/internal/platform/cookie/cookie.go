// Package cookie berisi helper cookie auth.
package cookie

import "net/http"

const (
	AccessTokenName  = "meme_access_token"
	RefreshTokenName = "meme_refresh_token"
)

func Set(w http.ResponseWriter, name, value string, maxAge int, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: false, // sengaja false agar document.cookie bisa membacanya
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func Clear(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: false,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})
}

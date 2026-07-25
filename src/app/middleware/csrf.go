package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

const csrfKey ctxKey = "csrf_token"

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func CSRFToken(r *http.Request) string {
	v, _ := r.Context().Value(csrfKey).(string)
	return v
}

func isMutation(r *http.Request) bool {
	return r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete
}

// isBrowserRequest reports whether a mutating request could have come from a
// browser carrying ambient credentials — the only case CSRF protects against.
//
// The Bearer check in CSRF only recognises an *authenticated* native client.
// An app built against an API that requires no auth has no token to send, so
// without this it would be forced down the browser path and fail every write.
//
// Browsers send Origin on every POST/PUT/DELETE — same-origin fetches and
// cross-site form submissions alike — so its absence on a mutation means the
// caller is not a browser. Browser protection is unaffected: a forged
// cross-site request still sends Origin, so it is still enforced and rejected.
//
// Cookie presence deliberately plays no part. A native client using a stock
// HTTP stack (URLSession, OkHttp) stores the csrf_token cookie set on its
// first GET and replays it automatically, so keying on cookies would drag
// every native app straight back into the browser path this exists to avoid.
//
// Only consulted for mutations — a GET may legitimately arrive without Origin,
// and that request still needs its token cookie issued.
func isBrowserRequest(r *http.Request) bool {
	return r.Header.Get("Origin") != ""
}

func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bearer-token requests (mobile clients) carry no cookies for this
		// origin, so a forged cross-site request can't replay them the way
		// it can a session cookie — CSRF doesn't apply to them.
		//
		// login_token is exempted by path for the same reason even though
		// it can't carry a Bearer header yet — it's the request that issues
		// the token, so there's nothing to attach. CSRF's threat model is a
		// browser auto-attaching credentials to a forged cross-site request;
		// a native app calling this endpoint directly was never reachable
		// that way in the first place.
		if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") || r.URL.Path == "/api/v1/auth/login_token" {
			next.ServeHTTP(w, r)
			return
		}

		token := ""
		if cookie, err := r.Cookie("csrf_token"); err == nil {
			token = cookie.Value
		} else {
			token = generateToken()
			http.SetCookie(w, &http.Cookie{
				Name:     "csrf_token",
				Value:    token,
				Path:     "/",
				HttpOnly: false,
				Secure:   secureCookies,
				SameSite: http.SameSiteStrictMode,
			})
		}

		ctx := context.WithValue(r.Context(), csrfKey, token)

		// Enforcement is skipped for callers that can't be a browser, but the
		// cookie above is still issued either way — a browser's first
		// same-origin GET sends no Origin and no cookie, and it must still
		// leave here holding a token to send back on its next mutation.
		if isMutation(r) && isBrowserRequest(r) {
			headerToken := r.Header.Get("X-CSRF-Token")
			if !hmac.Equal([]byte(token), []byte(headerToken)) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				w.Write([]byte(`{"ok":false,"error":"invalid CSRF token","code":"forbidden"}`))
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

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

// csrfCookieMaxAge is how long the double-submit cookie lives.
//
// It used to have no MaxAge at all, which made it a BROWSER-SESSION cookie
// while the session cookie is persistent. Those two lifetimes diverging is what
// puts a browser into the state "has a session, has no CSRF cookie" every time
// it is closed and reopened — see the hasSession block in CSRF below. Matching
// the session's own life removes the divergence rather than compensating for
// it. The value is not a secret and is deliberately readable by JS (api.js
// reads it to build the X-CSRF-Token header), so a longer life costs nothing.
//
// Keep this in step with the TTL handed to SetSession at login.
const csrfCookieMaxAge = 24 * 60 * 60

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func CSRFToken(r *http.Request) string {
	v, _ := r.Context().Value(csrfKey).(string)
	return v
}

// isSafeMethod reports whether a method is one a CSRF token is not required
// for — the three RFC 9110 calls "safe", meaning they are not expected to
// change state.
//
// It is the ONE definition used by both halves of CSRF below: the decision to
// mint a cookie, and the decision to verify a header. Two lists that must agree
// are two lists that eventually will not.
//
// This replaces the old `POST || PUT || DELETE` denylist from the upstream
// template. A denylist is the wrong SHAPE rather than a list with one entry
// missing — PATCH was never checked, and a method nobody thought of defaults
// to UNPROTECTED, silently. Inverted, the default flips: every method that
// exists or is ever added is verified without anyone having to remember to
// add it here.
func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
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

		// Read the INCOMING cookie once, before we might set one. Its
		// presence is the signal that this client is a browser participating
		// in the double-submit scheme (an earlier safe-method load set it).
		token := ""
		if cookie, err := r.Cookie("csrf_token"); err == nil {
			token = cookie.Value
		} else {
			// No cookie yet. Mint one so browser JS can read it, but only
			// issue it on safe methods — a mutating request that brought no
			// cookie is a native/non-browser client we must NOT force into
			// the scheme (doing so is the bug that 403'd every mobile write).
			token = generateToken()
			if isSafeMethod(r.Method) {
				http.SetCookie(w, &http.Cookie{
					Name:     "csrf_token",
					Value:    token,
					Path:     "/",
					HttpOnly: false,
					Secure:   secureCookies,
					SameSite: http.SameSiteStrictMode,
					MaxAge:   csrfCookieMaxAge,
				})
			}
		}

		ctx := context.WithValue(r.Context(), csrfKey, token)

		// ALLOWLIST THE SAFE METHODS, rather than listing the unsafe ones —
		// see isSafeMethod for why the shape of the list matters.
		if !isSafeMethod(r.Method) {
			// An ORIGIN HEADER means the caller is a browser, and a browser
			// is the only client CSRF defends against. This is the homelab
			// divergence from the upstream template: homelab is a Tailscale-
			// only deployment with NO auth, so there is no session cookie to
			// trigger the hasSession fail-closed branch the template adds —
			// its bearer/mobile clients are the native consumers, and they
			// are recognised by the absence of Origin rather than by a
			// session they do not have.
			//
			// Browser protection is unaffected: a forged cross-site request
			// sends Origin and is verified below; a same-origin fetch sends
			// Origin and carries the header api.js builds. What is skipped is
			// only the client that cannot be a browser.
			if isBrowserRequest(r) {
				headerToken := r.Header.Get("X-CSRF-Token")
				if !hmac.Equal([]byte(token), []byte(headerToken)) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusForbidden)
					w.Write([]byte(`{"ok":false,"error":"invalid CSRF token","code":"forbidden"}`))
					return
				}
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
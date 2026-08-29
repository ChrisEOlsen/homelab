package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// okHandler records that the request made it past the middleware.
func okHandler(reached *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestCSRFIssuesTokenCookieOnPlainGET(t *testing.T) {
	// A browser's first same-origin GET carries neither Origin nor a cookie.
	// It must still leave holding a token, or every later mutation fails.
	var reached bool
	req := httptest.NewRequest(http.MethodGet, "/api/v1/todos", nil)
	rec := httptest.NewRecorder()

	CSRF(okHandler(&reached)).ServeHTTP(rec, req)

	if !reached {
		t.Fatal("GET should reach the handler")
	}
	var found bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == "csrf_token" && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Error("GET must set a csrf_token cookie")
	}
}

func TestCSRFAllowsNativeMutations(t *testing.T) {
	// A native client sends no Origin. It may still replay the csrf_token
	// cookie, because stock HTTP stacks (URLSession, OkHttp) store it from
	// the first GET — that must not push it onto the browser path.
	cases := []struct {
		name   string
		method string
		cookie bool
	}{
		{"POST without cookie", http.MethodPost, false},
		{"POST with stored cookie", http.MethodPost, true},
		{"PUT with stored cookie", http.MethodPut, true},
		{"DELETE with stored cookie", http.MethodDelete, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var reached bool
			req := httptest.NewRequest(tc.method, "/api/v1/todos/1/toggle", nil)
			if tc.cookie {
				req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "stored-from-first-get"})
			}
			rec := httptest.NewRecorder()

			CSRF(okHandler(&reached)).ServeHTTP(rec, req)

			if !reached {
				t.Errorf("native %s must reach the handler, got %d", tc.method, rec.Code)
			}
		})
	}
}

func TestCSRFRejectsBrowserMutationsWithoutMatchingToken(t *testing.T) {
	cases := []struct {
		name   string
		origin string
		cookie string
		header string
	}{
		{"forged cross-site", "https://evil.example", "", ""},
		{"same-origin, no token", "http://localhost:1234", "", ""},
		{"cookie but no header", "http://localhost:1234", "abc123", ""},
		{"mismatched header", "http://localhost:1234", "abc123", "wrong"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var reached bool
			req := httptest.NewRequest(http.MethodPost, "/api/v1/todos/1/toggle", nil)
			req.Header.Set("Origin", tc.origin)
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "csrf_token", Value: tc.cookie})
			}
			if tc.header != "" {
				req.Header.Set("X-CSRF-Token", tc.header)
			}
			rec := httptest.NewRecorder()

			CSRF(okHandler(&reached)).ServeHTTP(rec, req)

			if reached {
				t.Error("browser mutation without a matching token must not reach the handler")
			}
			if rec.Code != http.StatusForbidden {
				t.Errorf("expected 403, got %d", rec.Code)
			}
		})
	}
}

func TestCSRFAllowsBrowserMutationWithMatchingToken(t *testing.T) {
	var reached bool
	req := httptest.NewRequest(http.MethodPost, "/api/v1/todos/1/toggle", nil)
	req.Header.Set("Origin", "http://localhost:1234")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "matching"})
	req.Header.Set("X-CSRF-Token", "matching")
	rec := httptest.NewRecorder()

	CSRF(okHandler(&reached)).ServeHTTP(rec, req)

	if !reached {
		t.Errorf("browser mutation with a matching token must pass, got %d", rec.Code)
	}
}

func TestCSRFExemptsBearerRequests(t *testing.T) {
	var reached bool
	req := httptest.NewRequest(http.MethodPost, "/api/v1/todos/1/toggle", nil)
	req.Header.Set("Origin", "http://localhost:1234")
	req.Header.Set("Authorization", "Bearer sometoken")
	rec := httptest.NewRecorder()

	CSRF(okHandler(&reached)).ServeHTTP(rec, req)

	if !reached {
		t.Errorf("bearer request must pass, got %d", rec.Code)
	}
}

// Every unsafe method the standard library names is verified — including ones
// nobody has written a handler for yet, and including PATCH in particular.
//
// This is the test the old denylist (`POST || PUT || DELETE`) could not have:
// it passed against the bug because PATCH was never checked at all. The
// allowlist's value is that it covers methods this file does not enumerate,
// so enumerating them here would defeat the point of the test. What it CAN
// assert is that the set of methods let through unverified is exactly the
// three safe ones, which is a property a future edit cannot widen by accident.
func TestCSRF_OnlySafeMethodsSkipVerification(t *testing.T) {
	safe := map[string]bool{http.MethodGet: true, http.MethodHead: true, http.MethodOptions: true}
	all := []string{
		http.MethodGet, http.MethodHead, http.MethodOptions,
		http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete,
		http.MethodConnect, http.MethodTrace,
		"PURGE", "LOCK", // methods no RFC in this app defines — must still fail closed
	}
	for _, m := range all {
		// Origin marks the caller as a browser, so every unsafe method must
		// be verified — a wrong header must 403 for each of them.
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(m, "/api/v1/things", nil)
		req.Header.Set("Origin", "http://localhost:1234")
		req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "abc123"})
		req.Header.Set("X-CSRF-Token", "wrong")
		CSRF(okHandler(new(bool))).ServeHTTP(rec, req)

		if safe[m] && rec.Code != http.StatusOK {
			t.Errorf("%s is safe: want 200, got %d", m, rec.Code)
		}
		if !safe[m] && rec.Code != http.StatusForbidden {
			t.Errorf("%s is unsafe: want 403 (verified), got %d", m, rec.Code)
		}
	}
}

// A mutating request that brought no csrf_token cookie is the native path and
// must NOT mint one — the cookie is only issued on safe methods, so a native
// write is never forced into the scheme later.
func TestCSRF_MutatingRequestDoesNotMintCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/things", nil)
	CSRF(okHandler(new(bool))).ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.Name == "csrf_token" {
			t.Fatal("a mutating request must not mint a csrf_token cookie")
		}
	}
}

// The csrf_token cookie outlives the browser session, so it cannot diverge from
// the session cookie on a restart and strand a legitimate browser in the
// fail-closed state.
func TestCSRF_MintedCookieIsPersistent(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	CSRF(okHandler(new(bool))).ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.Name == "csrf_token" {
			if c.MaxAge <= 0 {
				t.Fatalf("csrf_token must be persistent: MaxAge = %d", c.MaxAge)
			}
			return
		}
	}
	t.Fatal("GET did not mint a csrf_token cookie")
}
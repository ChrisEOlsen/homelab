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

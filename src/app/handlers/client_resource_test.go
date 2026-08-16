package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gova/app/cache"
	"gova/app/db"
	"gova/app/models"
)

func clientResourceSchema() string {
	return `CREATE TABLE clients (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		match_name TEXT NOT NULL,
		email TEXT,
		phone TEXT,
		rate_cents INTEGER NOT NULL,
		kind TEXT NOT NULL,
		is_active INTEGER NOT NULL,
		notes TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
}

// clientRouter mounts the five resource routes so path params resolve.
func clientRouter(testDB *db.DB, appCache *cache.Cache) chi.Router {
	r := chi.NewRouter()
	r.Get("/api/v1/clients", ClientListGET(testDB.Read, testDB.Write, appCache))
	r.Get("/api/v1/clients/{id}", ClientDetailGET(testDB.Read, testDB.Write, appCache))
	r.Post("/api/v1/clients", ClientCreatePOST(testDB.Read, testDB.Write, appCache))
	r.Put("/api/v1/clients/{id}", ClientUpdatePUT(testDB.Read, testDB.Write, appCache))
	r.Delete("/api/v1/clients/{id}", ClientDeleteDELETE(testDB.Read, testDB.Write, appCache))
	return r
}

func TestClientResourceCRUD(t *testing.T) {
	testDB := db.OpenTest(t, clientResourceSchema())
	appCache := cache.New()
	router := clientRouter(testDB, appCache)
	model := models.NewClientModel(testDB.Read, testDB.Write, appCache)
	emailTestVal := "test"
	phoneTestVal := "test"
	notesTestVal := "test"
	id, err := model.Create("test", "test", &emailTestVal, &phoneTestVal, int64(1), "test", true, &notesTestVal)
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	do := func(method, target, body string) *httptest.ResponseRecorder {
		var r *http.Request
		if body == "" {
			r = httptest.NewRequest(method, target, nil)
		} else {
			r = httptest.NewRequest(method, target, strings.NewReader(body))
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, r)
		return rec
	}

	// Detail: found and not-found.
	if rec := do(http.MethodGet, "/api/v1/clients/1", ""); rec.Code != 200 {
		t.Errorf("detail found: got %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodGet, "/api/v1/clients/999999", ""); rec.Code != 404 {
		t.Errorf("detail missing: got %d, want 404", rec.Code)
	}
	// Detail: non-numeric id -> 422.
	if rec := do(http.MethodGet, "/api/v1/clients/abc", ""); rec.Code != 422 {
		t.Errorf("detail bad id: got %d, want 422", rec.Code)
	}

	// Create.
	if rec := do(http.MethodPost, "/api/v1/clients", `{"name": "test", "match_name": "test", "email": "test", "phone": "test", "rate_cents": 1, "kind": "test", "is_active": true, "notes": "test"}`); rec.Code != 200 {
		t.Errorf("create: got %d, body %s", rec.Code, rec.Body.String())
	}

	// Update: existing and missing.
	if rec := do(http.MethodPut, "/api/v1/clients/1", `{"name": "test", "match_name": "test", "email": "test", "phone": "test", "rate_cents": 1, "kind": "test", "is_active": true, "notes": "test"}`); rec.Code != 200 {
		t.Errorf("update: got %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodPut, "/api/v1/clients/999999", `{"name": "test", "match_name": "test", "email": "test", "phone": "test", "rate_cents": 1, "kind": "test", "is_active": true, "notes": "test"}`); rec.Code != 404 {
		t.Errorf("update missing: got %d, want 404", rec.Code)
	}

	// List: valid sort ok, bogus sort -> 422.
	if rec := do(http.MethodGet, "/api/v1/clients?sort=-id", ""); rec.Code != 200 {
		t.Errorf("list sort: got %d", rec.Code)
	}
	if rec := do(http.MethodGet, "/api/v1/clients?sort=bogus", ""); rec.Code != 422 {
		t.Errorf("list bad sort: got %d, want 422", rec.Code)
	}

	// Delete.
	if rec := do(http.MethodDelete, "/api/v1/clients/1", ""); rec.Code != 200 {
		t.Errorf("delete: got %d", rec.Code)
	}
	_ = id
}

// clientResourceSchemaWithUniqueMatchName mirrors the real migration's
// idx_clients_match_name UNIQUE index, which clientResourceSchema omits so
// the CRUD test above can reuse the same match_name across seed and create.
func clientResourceSchemaWithUniqueMatchName() string {
	return clientResourceSchema() + `;
		CREATE UNIQUE INDEX idx_clients_match_name ON clients(match_name COLLATE NOCASE)`
}

// TestClientCreateDuplicateMatchNameReturns409 covers the hand-customized
// conflict handling in ClientCreatePOST: a duplicate match_name (a routine
// outcome of the review-queue flow, not an exceptional one) must answer 409
// via the shared isUniqueConstraintErr helper, not a generic 500.
func TestClientCreateDuplicateMatchNameReturns409(t *testing.T) {
	testDB := db.OpenTest(t, clientResourceSchemaWithUniqueMatchName())
	appCache := cache.New()
	router := clientRouter(testDB, appCache)

	do := func(method, target, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, r)
		return rec
	}

	body := `{"name": "Ofer Rubin", "match_name": "Ofer Rubin", "rate_cents": 10000, "kind": "independent", "is_active": true}`
	if rec := do(http.MethodPost, "/api/v1/clients", body); rec.Code != 200 {
		t.Fatalf("first create: got %d, body %s", rec.Code, rec.Body.String())
	}

	rec := do(http.MethodPost, "/api/v1/clients", body)
	if rec.Code != 409 {
		t.Fatalf("duplicate match_name: got %d, want 409, body %s", rec.Code, rec.Body.String())
	}

	var env struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Code != "conflict" {
		t.Errorf("code: got %q, want %q", env.Code, "conflict")
	}
}

// TestClientUpdateDuplicateMatchNameReturns409 covers the same normalization
// on the update path: renaming one client's match_name onto another's is a
// UNIQUE violation and must also answer 409, not 500.
func TestClientUpdateDuplicateMatchNameReturns409(t *testing.T) {
	testDB := db.OpenTest(t, clientResourceSchemaWithUniqueMatchName())
	appCache := cache.New()
	router := clientRouter(testDB, appCache)
	model := models.NewClientModel(testDB.Read, testDB.Write, appCache)

	if _, err := model.Create("Ofer Rubin", "Ofer Rubin", nil, nil, 10000, "independent", true, nil); err != nil {
		t.Fatalf("seed first client: %v", err)
	}
	id2, err := model.Create("Ran Rubin", "Ran Rubin", nil, nil, 6000, "independent", true, nil)
	if err != nil {
		t.Fatalf("seed second client: %v", err)
	}

	do := func(method, target, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, r)
		return rec
	}

	body := `{"name": "Ran Rubin", "match_name": "Ofer Rubin", "rate_cents": 6000, "kind": "independent", "is_active": true}`
	rec := do(http.MethodPut, "/api/v1/clients/"+strconv.FormatInt(id2, 10), body)
	if rec.Code != 409 {
		t.Fatalf("update onto duplicate match_name: got %d, want 409, body %s", rec.Code, rec.Body.String())
	}
}

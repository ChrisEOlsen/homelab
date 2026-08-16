package handlers

import (
	"net/http"
	"net/http/httptest"
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

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

func training_sessionResourceSchema() string {
	return `CREATE TABLE training_sessions (
		id INTEGER PRIMARY KEY,
		uid TEXT NOT NULL,
		source TEXT NOT NULL,
		client_name TEXT NOT NULL,
		client_id INTEGER,
		service TEXT,
		session_date TEXT NOT NULL,
		start_at TEXT NOT NULL,
		end_at TEXT NOT NULL,
		duration_min INTEGER NOT NULL,
		amount_cents INTEGER NOT NULL,
		rate_source TEXT NOT NULL,
		override_cents INTEGER,
		status TEXT NOT NULL,
		needs_review INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
}

// training_sessionRouter mounts the five resource routes so path params resolve.
func training_sessionRouter(testDB *db.DB, appCache *cache.Cache) chi.Router {
	r := chi.NewRouter()
	r.Get("/api/v1/training_sessions", TrainingSessionListGET(testDB.Read, testDB.Write, appCache))
	r.Get("/api/v1/training_sessions/{id}", TrainingSessionDetailGET(testDB.Read, testDB.Write, appCache))
	r.Post("/api/v1/training_sessions", TrainingSessionCreatePOST(testDB.Read, testDB.Write, appCache))
	r.Put("/api/v1/training_sessions/{id}", TrainingSessionUpdatePUT(testDB.Read, testDB.Write, appCache))
	r.Delete("/api/v1/training_sessions/{id}", TrainingSessionDeleteDELETE(testDB.Read, testDB.Write, appCache))
	return r
}

func TestTrainingSessionResourceCRUD(t *testing.T) {
	testDB := db.OpenTest(t, training_sessionResourceSchema())
	appCache := cache.New()
	router := training_sessionRouter(testDB, appCache)
	model := models.NewTrainingSessionModel(testDB.Read, testDB.Write, appCache)
	client_idTestVal := int64(1)
	serviceTestVal := "test"
	override_centsTestVal := int64(1)
	id, err := model.Create("test", "test", "test", &client_idTestVal, &serviceTestVal, "test", "test", "test", int64(1), int64(1), "test", &override_centsTestVal, "test", true)
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
	if rec := do(http.MethodGet, "/api/v1/training_sessions/1", ""); rec.Code != 200 {
		t.Errorf("detail found: got %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodGet, "/api/v1/training_sessions/999999", ""); rec.Code != 404 {
		t.Errorf("detail missing: got %d, want 404", rec.Code)
	}
	// Detail: non-numeric id -> 422.
	if rec := do(http.MethodGet, "/api/v1/training_sessions/abc", ""); rec.Code != 422 {
		t.Errorf("detail bad id: got %d, want 422", rec.Code)
	}

	// Create.
	if rec := do(http.MethodPost, "/api/v1/training_sessions", `{"uid": "test", "source": "test", "client_name": "test", "client_id": 1, "service": "test", "session_date": "test", "start_at": "test", "end_at": "test", "duration_min": 1, "amount_cents": 1, "rate_source": "test", "override_cents": 1, "status": "test", "needs_review": true}`); rec.Code != 200 {
		t.Errorf("create: got %d, body %s", rec.Code, rec.Body.String())
	}

	// Update: existing and missing.
	if rec := do(http.MethodPut, "/api/v1/training_sessions/1", `{"uid": "test", "source": "test", "client_name": "test", "client_id": 1, "service": "test", "session_date": "test", "start_at": "test", "end_at": "test", "duration_min": 1, "amount_cents": 1, "rate_source": "test", "override_cents": 1, "status": "test", "needs_review": true}`); rec.Code != 200 {
		t.Errorf("update: got %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodPut, "/api/v1/training_sessions/999999", `{"uid": "test", "source": "test", "client_name": "test", "client_id": 1, "service": "test", "session_date": "test", "start_at": "test", "end_at": "test", "duration_min": 1, "amount_cents": 1, "rate_source": "test", "override_cents": 1, "status": "test", "needs_review": true}`); rec.Code != 404 {
		t.Errorf("update missing: got %d, want 404", rec.Code)
	}

	// List: valid sort ok, bogus sort -> 422.
	if rec := do(http.MethodGet, "/api/v1/training_sessions?sort=-id", ""); rec.Code != 200 {
		t.Errorf("list sort: got %d", rec.Code)
	}
	if rec := do(http.MethodGet, "/api/v1/training_sessions?sort=bogus", ""); rec.Code != 422 {
		t.Errorf("list bad sort: got %d, want 422", rec.Code)
	}

	// Delete.
	if rec := do(http.MethodDelete, "/api/v1/training_sessions/1", ""); rec.Code != 200 {
		t.Errorf("delete: got %d", rec.Code)
	}
	_ = id
}

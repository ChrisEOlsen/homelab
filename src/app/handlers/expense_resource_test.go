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

func expenseResourceSchema() string {
	return `CREATE TABLE expenses (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		amount_cents INTEGER NOT NULL,
		category TEXT,
		status TEXT NOT NULL,
		incurred_on TEXT NOT NULL,
		notes TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
}

// expenseRouter mounts the five resource routes so path params resolve.
func expenseRouter(testDB *db.DB, appCache *cache.Cache) chi.Router {
	r := chi.NewRouter()
	r.Get("/api/v1/expenses", ExpenseListGET(testDB.Read, testDB.Write, appCache))
	r.Get("/api/v1/expenses/{id}", ExpenseDetailGET(testDB.Read, testDB.Write, appCache))
	r.Post("/api/v1/expenses", ExpenseCreatePOST(testDB.Read, testDB.Write, appCache))
	r.Put("/api/v1/expenses/{id}", ExpenseUpdatePUT(testDB.Read, testDB.Write, appCache))
	r.Delete("/api/v1/expenses/{id}", ExpenseDeleteDELETE(testDB.Read, testDB.Write, appCache))
	return r
}

func TestExpenseResourceCRUD(t *testing.T) {
	testDB := db.OpenTest(t, expenseResourceSchema())
	appCache := cache.New()
	router := expenseRouter(testDB, appCache)
	model := models.NewExpenseModel(testDB.Read, testDB.Write, appCache)
	categoryTestVal := "test"
	notesTestVal := "test"
	id, err := model.Create("test", int64(1), &categoryTestVal, "test", "test", &notesTestVal)
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
	if rec := do(http.MethodGet, "/api/v1/expenses/1", ""); rec.Code != 200 {
		t.Errorf("detail found: got %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodGet, "/api/v1/expenses/999999", ""); rec.Code != 404 {
		t.Errorf("detail missing: got %d, want 404", rec.Code)
	}
	// Detail: non-numeric id -> 422.
	if rec := do(http.MethodGet, "/api/v1/expenses/abc", ""); rec.Code != 422 {
		t.Errorf("detail bad id: got %d, want 422", rec.Code)
	}

	// Create.
	if rec := do(http.MethodPost, "/api/v1/expenses", `{"name": "test", "amount_cents": 1, "category": "test", "status": "test", "incurred_on": "test", "notes": "test"}`); rec.Code != 200 {
		t.Errorf("create: got %d, body %s", rec.Code, rec.Body.String())
	}

	// Update: existing and missing.
	if rec := do(http.MethodPut, "/api/v1/expenses/1", `{"name": "test", "amount_cents": 1, "category": "test", "status": "test", "incurred_on": "test", "notes": "test"}`); rec.Code != 200 {
		t.Errorf("update: got %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodPut, "/api/v1/expenses/999999", `{"name": "test", "amount_cents": 1, "category": "test", "status": "test", "incurred_on": "test", "notes": "test"}`); rec.Code != 404 {
		t.Errorf("update missing: got %d, want 404", rec.Code)
	}

	// List: valid sort ok, bogus sort -> 422.
	if rec := do(http.MethodGet, "/api/v1/expenses?sort=-id", ""); rec.Code != 200 {
		t.Errorf("list sort: got %d", rec.Code)
	}
	if rec := do(http.MethodGet, "/api/v1/expenses?sort=bogus", ""); rec.Code != 422 {
		t.Errorf("list bad sort: got %d, want 422", rec.Code)
	}

	// Delete.
	if rec := do(http.MethodDelete, "/api/v1/expenses/1", ""); rec.Code != 200 {
		t.Errorf("delete: got %d", rec.Code)
	}
	_ = id
}

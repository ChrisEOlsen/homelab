package handlers

import (
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

func rate_ruleResourceSchema() string {
	return `CREATE TABLE rate_rules (
		id INTEGER PRIMARY KEY,
		duration_min INTEGER NOT NULL,
		amount_cents INTEGER NOT NULL,
		label TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
}

// rate_ruleRouter mounts the five resource routes so path params resolve.
func rate_ruleRouter(testDB *db.DB, appCache *cache.Cache) chi.Router {
	r := chi.NewRouter()
	r.Get("/api/v1/rate_rules", RateRuleListGET(testDB.Read, testDB.Write, appCache))
	r.Get("/api/v1/rate_rules/{id}", RateRuleDetailGET(testDB.Read, testDB.Write, appCache))
	r.Post("/api/v1/rate_rules", RateRuleCreatePOST(testDB.Read, testDB.Write, appCache))
	r.Put("/api/v1/rate_rules/{id}", RateRuleUpdatePUT(testDB.Read, testDB.Write, appCache))
	r.Delete("/api/v1/rate_rules/{id}", RateRuleDeleteDELETE(testDB.Read, testDB.Write, appCache))
	return r
}

func TestRateRuleResourceCRUD(t *testing.T) {
	testDB := db.OpenTest(t, rate_ruleResourceSchema())
	appCache := cache.New()
	router := rate_ruleRouter(testDB, appCache)
	model := models.NewRateRuleModel(testDB.Read, testDB.Write, appCache)
	labelTestVal := "test"
	id, err := model.Create(int64(1), int64(1), &labelTestVal)
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
	if rec := do(http.MethodGet, "/api/v1/rate_rules/1", ""); rec.Code != 200 {
		t.Errorf("detail found: got %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodGet, "/api/v1/rate_rules/999999", ""); rec.Code != 404 {
		t.Errorf("detail missing: got %d, want 404", rec.Code)
	}
	// Detail: non-numeric id -> 422.
	if rec := do(http.MethodGet, "/api/v1/rate_rules/abc", ""); rec.Code != 422 {
		t.Errorf("detail bad id: got %d, want 422", rec.Code)
	}

	// Create.
	if rec := do(http.MethodPost, "/api/v1/rate_rules", `{"duration_min": 1, "amount_cents": 1, "label": "test"}`); rec.Code != 200 {
		t.Errorf("create: got %d, body %s", rec.Code, rec.Body.String())
	}

	// Update: existing and missing.
	if rec := do(http.MethodPut, "/api/v1/rate_rules/1", `{"duration_min": 1, "amount_cents": 1, "label": "test"}`); rec.Code != 200 {
		t.Errorf("update: got %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodPut, "/api/v1/rate_rules/999999", `{"duration_min": 1, "amount_cents": 1, "label": "test"}`); rec.Code != 404 {
		t.Errorf("update missing: got %d, want 404", rec.Code)
	}

	// List: valid sort ok, bogus sort -> 422.
	if rec := do(http.MethodGet, "/api/v1/rate_rules?sort=-id", ""); rec.Code != 200 {
		t.Errorf("list sort: got %d", rec.Code)
	}
	if rec := do(http.MethodGet, "/api/v1/rate_rules?sort=bogus", ""); rec.Code != 422 {
		t.Errorf("list bad sort: got %d, want 422", rec.Code)
	}

	// Delete.
	if rec := do(http.MethodDelete, "/api/v1/rate_rules/1", ""); rec.Code != 200 {
		t.Errorf("delete: got %d", rec.Code)
	}
	_ = id
}

// rate_ruleResourceSchemaWithUniqueDuration mirrors the real migration's
// `duration_min INTEGER NOT NULL UNIQUE`, which rate_ruleResourceSchema omits
// so the CRUD test above can reuse duration_min: 1 across seed and create.
func rate_ruleResourceSchemaWithUniqueDuration() string {
	return `CREATE TABLE rate_rules (
		id INTEGER PRIMARY KEY,
		duration_min INTEGER NOT NULL UNIQUE,
		amount_cents INTEGER NOT NULL,
		label TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
}

// TestRateRuleCreateDuplicateDurationReturns409 covers the hand-customized
// conflict handling in RateRuleCreatePOST: a duplicate duration_min must
// answer 409 via the shared isUniqueConstraintErr helper, not a generic 500.
func TestRateRuleCreateDuplicateDurationReturns409(t *testing.T) {
	testDB := db.OpenTest(t, rate_ruleResourceSchemaWithUniqueDuration())
	appCache := cache.New()
	router := rate_ruleRouter(testDB, appCache)

	do := func(method, target, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, r)
		return rec
	}

	body := `{"duration_min": 45, "amount_cents": 5000, "label": "45 minutes"}`
	if rec := do(http.MethodPost, "/api/v1/rate_rules", body); rec.Code != 200 {
		t.Fatalf("first create: got %d, body %s", rec.Code, rec.Body.String())
	}

	rec := do(http.MethodPost, "/api/v1/rate_rules", body)
	if rec.Code != 409 {
		t.Fatalf("duplicate duration_min: got %d, want 409, body %s", rec.Code, rec.Body.String())
	}
}

// TestRateRuleUpdateDuplicateDurationReturns409 covers the same handling on
// the update path: changing one rule's duration onto another's is a UNIQUE
// violation and must also answer 409, not 500.
func TestRateRuleUpdateDuplicateDurationReturns409(t *testing.T) {
	testDB := db.OpenTest(t, rate_ruleResourceSchemaWithUniqueDuration())
	appCache := cache.New()
	router := rate_ruleRouter(testDB, appCache)
	model := models.NewRateRuleModel(testDB.Read, testDB.Write, appCache)

	if _, err := model.Create(45, 5000, nil); err != nil {
		t.Fatalf("seed first rule: %v", err)
	}
	id2, err := model.Create(60, 6000, nil)
	if err != nil {
		t.Fatalf("seed second rule: %v", err)
	}

	do := func(method, target, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, r)
		return rec
	}

	body := `{"duration_min": 45, "amount_cents": 6000}`
	rec := do(http.MethodPut, "/api/v1/rate_rules/"+strconv.FormatInt(id2, 10), body)
	if rec.Code != 409 {
		t.Fatalf("update onto duplicate duration_min: got %d, want 409, body %s", rec.Code, rec.Body.String())
	}
}

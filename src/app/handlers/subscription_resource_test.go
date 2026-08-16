package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"gova/app/cache"
	"gova/app/db"
	"gova/app/models"
)

func subscriptionResourceSchema() string {
	return `CREATE TABLE subscriptions (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		amount_cents INTEGER NOT NULL,
		cadence TEXT NOT NULL,
		billing_day INTEGER,
		is_active INTEGER NOT NULL,
		started_on TEXT NOT NULL,
		ended_on TEXT,
		notes TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
}

// subscriptionRouter mounts the five resource routes so path params resolve.
func subscriptionRouter(testDB *db.DB, appCache *cache.Cache) chi.Router {
	r := chi.NewRouter()
	r.Get("/api/v1/subscriptions", SubscriptionListGET(testDB.Read, testDB.Write, appCache))
	r.Get("/api/v1/subscriptions/{id}", SubscriptionDetailGET(testDB.Read, testDB.Write, appCache))
	r.Post("/api/v1/subscriptions", SubscriptionCreatePOST(testDB.Read, testDB.Write, appCache))
	r.Put("/api/v1/subscriptions/{id}", SubscriptionUpdatePUT(testDB.Read, testDB.Write, appCache))
	r.Delete("/api/v1/subscriptions/{id}", SubscriptionDeleteDELETE(testDB.Read, testDB.Write, appCache))
	return r
}

func TestSubscriptionResourceCRUD(t *testing.T) {
	testDB := db.OpenTest(t, subscriptionResourceSchema())
	appCache := cache.New()
	router := subscriptionRouter(testDB, appCache)
	model := models.NewSubscriptionModel(testDB.Read, testDB.Write, appCache)
	billing_dayTestVal := int64(1)
	ended_onTestVal := "test"
	notesTestVal := "test"
	id, err := model.Create("test", int64(1), "test", &billing_dayTestVal, true, "test", &ended_onTestVal, &notesTestVal)
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
	if rec := do(http.MethodGet, "/api/v1/subscriptions/1", ""); rec.Code != 200 {
		t.Errorf("detail found: got %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodGet, "/api/v1/subscriptions/999999", ""); rec.Code != 404 {
		t.Errorf("detail missing: got %d, want 404", rec.Code)
	}
	// Detail: non-numeric id -> 422.
	if rec := do(http.MethodGet, "/api/v1/subscriptions/abc", ""); rec.Code != 422 {
		t.Errorf("detail bad id: got %d, want 422", rec.Code)
	}

	// Create.
	if rec := do(http.MethodPost, "/api/v1/subscriptions", `{"name": "test", "amount_cents": 1, "cadence": "monthly", "billing_day": 1, "is_active": true, "started_on": "test", "ended_on": "test", "notes": "test"}`); rec.Code != 200 {
		t.Errorf("create: got %d, body %s", rec.Code, rec.Body.String())
	}

	// Update: existing and missing.
	if rec := do(http.MethodPut, "/api/v1/subscriptions/1", `{"name": "test", "amount_cents": 1, "cadence": "monthly", "billing_day": 1, "is_active": true, "started_on": "test", "ended_on": "test", "notes": "test"}`); rec.Code != 200 {
		t.Errorf("update: got %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodPut, "/api/v1/subscriptions/999999", `{"name": "test", "amount_cents": 1, "cadence": "monthly", "billing_day": 1, "is_active": true, "started_on": "test", "ended_on": "test", "notes": "test"}`); rec.Code != 404 {
		t.Errorf("update missing: got %d, want 404", rec.Code)
	}

	// List: valid sort ok, bogus sort -> 422.
	if rec := do(http.MethodGet, "/api/v1/subscriptions?sort=-id", ""); rec.Code != 200 {
		t.Errorf("list sort: got %d", rec.Code)
	}
	if rec := do(http.MethodGet, "/api/v1/subscriptions?sort=bogus", ""); rec.Code != 422 {
		t.Errorf("list bad sort: got %d, want 422", rec.Code)
	}

	// Delete.
	if rec := do(http.MethodDelete, "/api/v1/subscriptions/1", ""); rec.Code != 200 {
		t.Errorf("delete: got %d", rec.Code)
	}
	_ = id
}

// TestSubscriptionCreateDefaultsCadence covers the hand-customized defaulting
// in SubscriptionCreatePOST: an omitted cadence becomes "monthly".
func TestSubscriptionCreateDefaultsCadence(t *testing.T) {
	testDB := db.OpenTest(t, subscriptionResourceSchema())
	appCache := cache.New()
	router := subscriptionRouter(testDB, appCache)
	model := models.NewSubscriptionModel(testDB.Read, testDB.Write, appCache)

	do := func(method, target, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, r)
		return rec
	}

	rec := do(http.MethodPost, "/api/v1/subscriptions", `{"name": "Spotify", "amount_cents": 1199, "started_on": "2026-01-01"}`)
	if rec.Code != 200 {
		t.Fatalf("create: got %d, body %s", rec.Code, rec.Body.String())
	}

	item, err := model.Find(1)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if item.Cadence != "monthly" {
		t.Errorf("cadence: got %q, want %q", item.Cadence, "monthly")
	}
}

// TestSubscriptionUpdateDeactivateStampsEndedOn covers the hand-customized
// stamping in SubscriptionUpdatePUT: switching is_active off with no ended_on
// stamps today's date, since month membership must be a pure date-range
// question and never read is_active directly.
func TestSubscriptionUpdateDeactivateStampsEndedOn(t *testing.T) {
	testDB := db.OpenTest(t, subscriptionResourceSchema())
	appCache := cache.New()
	router := subscriptionRouter(testDB, appCache)
	model := models.NewSubscriptionModel(testDB.Read, testDB.Write, appCache)

	do := func(method, target, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, r)
		return rec
	}

	today := time.Now().Format("2006-01-02")

	id, err := model.Create("Old gym", 8000, "monthly", nil, true, "2025-01-01", nil, nil)
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	rec := do(http.MethodPut, "/api/v1/subscriptions/1", `{"name": "Old gym", "amount_cents": 8000, "cadence": "monthly", "is_active": false, "started_on": "2025-01-01"}`)
	if rec.Code != 200 {
		t.Fatalf("deactivate: got %d, body %s", rec.Code, rec.Body.String())
	}

	item, err := model.Find(id)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if item.EndedOn == nil || *item.EndedOn != today {
		t.Errorf("ended_on: got %v, want %q", item.EndedOn, today)
	}
}

// TestSubscriptionUpdateReactivateClearsEndedOn covers the hand-customized
// clearing in SubscriptionUpdatePUT: switching is_active back on clears
// ended_on to NULL.
func TestSubscriptionUpdateReactivateClearsEndedOn(t *testing.T) {
	testDB := db.OpenTest(t, subscriptionResourceSchema())
	appCache := cache.New()
	router := subscriptionRouter(testDB, appCache)
	model := models.NewSubscriptionModel(testDB.Read, testDB.Write, appCache)

	do := func(method, target, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, r)
		return rec
	}

	endedOn := "2026-07-15"
	id, err := model.Create("Old gym", 8000, "monthly", nil, false, "2025-01-01", &endedOn, nil)
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	rec := do(http.MethodPut, "/api/v1/subscriptions/1", `{"name": "Old gym", "amount_cents": 8000, "cadence": "monthly", "is_active": true, "started_on": "2025-01-01"}`)
	if rec.Code != 200 {
		t.Fatalf("reactivate: got %d, body %s", rec.Code, rec.Body.String())
	}

	item, err := model.Find(id)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if item.EndedOn != nil {
		t.Errorf("ended_on: got %v, want nil", item.EndedOn)
	}
}

// TestSubscriptionRejectsInvalidCadence covers the added floor under cadence:
// an unrecognized cadence is rejected with 422 on both create and update, and
// the row is left unchanged. Without this, monthlyEquivalent's switch default
// silently treats an unrecognized cadence as monthly, which would understate
// or overstate real money with no error surfaced.
func TestSubscriptionRejectsInvalidCadence(t *testing.T) {
	testDB := db.OpenTest(t, subscriptionResourceSchema())
	appCache := cache.New()
	router := subscriptionRouter(testDB, appCache)
	model := models.NewSubscriptionModel(testDB.Read, testDB.Write, appCache)

	do := func(method, target, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, r)
		return rec
	}

	// Invalid cadence on create.
	rec := do(http.MethodPost, "/api/v1/subscriptions", `{"name": "Bad", "amount_cents": 100, "cadence": "daily", "started_on": "2026-01-01"}`)
	if rec.Code != 422 {
		t.Errorf("create invalid cadence: got %d, want 422, body %s", rec.Code, rec.Body.String())
	}

	id, err := model.Create("Yearly thing", 12000, "yearly", nil, true, "2026-01-01", nil, nil)
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	// Invalid cadence on update.
	rec = do(http.MethodPut, "/api/v1/subscriptions/1", `{"name": "Yearly thing", "amount_cents": 12000, "cadence": "daily", "is_active": true, "started_on": "2026-01-01"}`)
	if rec.Code != 422 {
		t.Errorf("update invalid cadence: got %d, want 422, body %s", rec.Code, rec.Body.String())
	}

	// Omitted cadence on update (no default on PUT) -> also 422.
	rec = do(http.MethodPut, "/api/v1/subscriptions/1", `{"name": "Yearly thing", "amount_cents": 12000, "is_active": true, "started_on": "2026-01-01"}`)
	if rec.Code != 422 {
		t.Errorf("update omitted cadence: got %d, want 422, body %s", rec.Code, rec.Body.String())
	}

	item, err := model.Find(id)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if item.Cadence != "yearly" {
		t.Errorf("row mutated: cadence got %q, want unchanged %q", item.Cadence, "yearly")
	}
	if item.AmountCents != 12000 {
		t.Errorf("row mutated: amount_cents got %d, want unchanged %d", item.AmountCents, 12000)
	}
}

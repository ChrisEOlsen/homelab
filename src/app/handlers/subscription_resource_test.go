package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gova/app/cache"
	"gova/app/calendar"
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
	ended_onTestVal := "2026-02-01"
	notesTestVal := "test"
	id, err := model.Create("test", int64(1), "test", &billing_dayTestVal, true, "2026-01-01", &ended_onTestVal, &notesTestVal)
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
	if rec := do(http.MethodPost, "/api/v1/subscriptions", `{"name": "test", "amount_cents": 1, "cadence": "monthly", "billing_day": 1, "is_active": true, "started_on": "2026-01-01", "ended_on": "2026-02-01", "notes": "test"}`); rec.Code != 200 {
		t.Errorf("create: got %d, body %s", rec.Code, rec.Body.String())
	}

	// Update: existing and missing.
	if rec := do(http.MethodPut, "/api/v1/subscriptions/1", `{"name": "test", "amount_cents": 1, "cadence": "monthly", "billing_day": 1, "is_active": true, "started_on": "2026-01-01", "ended_on": "2026-02-01", "notes": "test"}`); rec.Code != 200 {
		t.Errorf("update: got %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodPut, "/api/v1/subscriptions/999999", `{"name": "test", "amount_cents": 1, "cadence": "monthly", "billing_day": 1, "is_active": true, "started_on": "2026-01-01", "ended_on": "2026-02-01", "notes": "test"}`); rec.Code != 404 {
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

	today := calendar.Now().Format("2006-01-02")

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

// TestSubscriptionCreateInactiveStampsEndedOn covers the hand-customized
// normalization in SubscriptionCreatePOST (FIX B): POSTing is_active:false
// with no ended_on must stamp today's date immediately, the same
// normalization SubscriptionUpdatePUT already applied. Without it, month
// membership -- read purely from started_on/ended_on -- would count the
// subscription as live in every month from started_on onward forever, even
// though the UI shows it struck through from creation.
func TestSubscriptionCreateInactiveStampsEndedOn(t *testing.T) {
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

	today := calendar.Now().Format("2006-01-02")

	rec := do(http.MethodPost, "/api/v1/subscriptions", `{"name": "Cancelled gym", "amount_cents": 8000, "cadence": "monthly", "is_active": false, "started_on": "2025-01-01"}`)
	if rec.Code != 200 {
		t.Fatalf("create: got %d, body %s", rec.Code, rec.Body.String())
	}

	item, err := model.Find(1)
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

// TestSubscriptionUpdateRejectsInvalidStartedOn covers the added floor under
// started_on on the full-replace PUT: unlike create, update has no default,
// so an omitted or malformed started_on must be a 422 — the caller has to say
// what it means rather than corrupt a money calculation (see
// MonthlyEquivalentFor's slice-bounds panic on a short started_on).
func TestSubscriptionUpdateRejectsInvalidStartedOn(t *testing.T) {
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

	id, err := model.Create("Streaming", 1500, "monthly", nil, true, "2026-01-01", nil, nil)
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	// started_on omitted entirely -> 422.
	rec := do(http.MethodPut, "/api/v1/subscriptions/1", `{"name": "Streaming", "amount_cents": 1500, "cadence": "monthly", "is_active": true}`)
	if rec.Code != 422 {
		t.Errorf("omitted started_on: got %d, want 422, body %s", rec.Code, rec.Body.String())
	}

	// started_on present but impossible calendar date -> 422.
	rec = do(http.MethodPut, "/api/v1/subscriptions/1", `{"name": "Streaming", "amount_cents": 1500, "cadence": "monthly", "is_active": true, "started_on": "2026-13-45"}`)
	if rec.Code != 422 {
		t.Errorf("malformed started_on: got %d, want 422, body %s", rec.Code, rec.Body.String())
	}

	// ended_on present but malformed -> 422. is_active is false so the
	// deactivate-stamping branch does not overwrite the supplied ended_on
	// before validation runs.
	rec = do(http.MethodPut, "/api/v1/subscriptions/1", `{"name": "Streaming", "amount_cents": 1500, "cadence": "monthly", "is_active": false, "started_on": "2026-01-01", "ended_on": "not-a-date"}`)
	if rec.Code != 422 {
		t.Errorf("malformed ended_on: got %d, want 422, body %s", rec.Code, rec.Body.String())
	}

	item, err := model.Find(id)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if item.StartedOn != "2026-01-01" {
		t.Errorf("row mutated: started_on got %q, want unchanged %q", item.StartedOn, "2026-01-01")
	}
	if item.EndedOn != nil {
		t.Errorf("row mutated: ended_on got %v, want unchanged nil", item.EndedOn)
	}
}

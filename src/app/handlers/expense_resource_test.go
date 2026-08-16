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
	if rec := do(http.MethodPost, "/api/v1/expenses", `{"name": "test", "amount_cents": 1, "category": "test", "status": "planned", "incurred_on": "test", "notes": "test"}`); rec.Code != 200 {
		t.Errorf("create: got %d, body %s", rec.Code, rec.Body.String())
	}

	// Update: existing and missing.
	if rec := do(http.MethodPut, "/api/v1/expenses/1", `{"name": "test", "amount_cents": 1, "category": "test", "status": "planned", "incurred_on": "test", "notes": "test"}`); rec.Code != 200 {
		t.Errorf("update: got %d, body %s", rec.Code, rec.Body.String())
	}
	if rec := do(http.MethodPut, "/api/v1/expenses/999999", `{"name": "test", "amount_cents": 1, "category": "test", "status": "planned", "incurred_on": "test", "notes": "test"}`); rec.Code != 404 {
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

// TestExpenseCreateDefaultsStatusAndDate covers the hand-customized defaulting
// in ExpenseCreatePOST: an omitted status becomes "planned" and an omitted
// incurred_on becomes today.
func TestExpenseCreateDefaultsStatusAndDate(t *testing.T) {
	testDB := db.OpenTest(t, expenseResourceSchema())
	appCache := cache.New()
	router := expenseRouter(testDB, appCache)
	model := models.NewExpenseModel(testDB.Read, testDB.Write, appCache)

	do := func(method, target, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, r)
		return rec
	}

	today := time.Now().Format("2006-01-02")

	rec := do(http.MethodPost, "/api/v1/expenses", `{"name": "gizmo", "amount_cents": 500}`)
	if rec.Code != 200 {
		t.Fatalf("create: got %d, body %s", rec.Code, rec.Body.String())
	}

	item, err := model.Find(1)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if item.Status != "planned" {
		t.Errorf("status: got %q, want %q", item.Status, "planned")
	}
	if item.IncurredOn != today {
		t.Errorf("incurred_on: got %q, want %q", item.IncurredOn, today)
	}
}

// TestExpenseUpdateBoughtRestampsDate covers the hand-customized re-stamping
// in ExpenseUpdatePUT: marking an item bought with no incurred_on moves it to
// today, but an explicit incurred_on always wins.
func TestExpenseUpdateBoughtRestampsDate(t *testing.T) {
	testDB := db.OpenTest(t, expenseResourceSchema())
	appCache := cache.New()
	router := expenseRouter(testDB, appCache)
	model := models.NewExpenseModel(testDB.Read, testDB.Write, appCache)

	do := func(method, target, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, r)
		return rec
	}

	today := time.Now().Format("2006-01-02")

	// Case 1: bought with no incurred_on -> re-stamped to today.
	id1, err := model.Create("widget", 1000, nil, "planned", "2020-01-01", nil)
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	rec := do(http.MethodPut, "/api/v1/expenses/1", `{"name": "widget", "amount_cents": 1000, "status": "bought", "incurred_on": ""}`)
	if rec.Code != 200 {
		t.Fatalf("update bought no date: got %d, body %s", rec.Code, rec.Body.String())
	}
	item1, err := model.Find(id1)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if item1.IncurredOn != today {
		t.Errorf("restamp: got %q, want today %q", item1.IncurredOn, today)
	}
	if item1.Status != "bought" {
		t.Errorf("status: got %q, want %q", item1.Status, "bought")
	}

	// Case 2: bought with an explicit incurred_on -> the supplied date wins.
	id2, err := model.Create("gadget", 2000, nil, "planned", "2020-01-01", nil)
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}
	rec = do(http.MethodPut, "/api/v1/expenses/2", `{"name": "gadget", "amount_cents": 2000, "status": "bought", "incurred_on": "2021-06-15"}`)
	if rec.Code != 200 {
		t.Fatalf("update bought explicit date: got %d, body %s", rec.Code, rec.Body.String())
	}
	item2, err := model.Find(id2)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if item2.IncurredOn != "2021-06-15" {
		t.Errorf("explicit date: got %q, want %q", item2.IncurredOn, "2021-06-15")
	}
}

// TestExpenseUpdateRejectsInvalidStatus covers the added floor under status
// on the full-replace PUT: a status outside {planned, bought}, or an omitted
// status, is rejected with 422 and the row is left unchanged.
func TestExpenseUpdateRejectsInvalidStatus(t *testing.T) {
	testDB := db.OpenTest(t, expenseResourceSchema())
	appCache := cache.New()
	router := expenseRouter(testDB, appCache)
	model := models.NewExpenseModel(testDB.Read, testDB.Write, appCache)

	do := func(method, target, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, target, strings.NewReader(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, r)
		return rec
	}

	id, err := model.Create("thingamajig", 300, nil, "bought", "2020-05-05", nil)
	if err != nil {
		t.Fatalf("seed Create: %v", err)
	}

	// Invalid status value.
	rec := do(http.MethodPut, "/api/v1/expenses/1", `{"name": "thingamajig", "amount_cents": 300, "status": "wishlist", "incurred_on": "2020-05-05"}`)
	if rec.Code != 422 {
		t.Errorf("invalid status: got %d, want 422, body %s", rec.Code, rec.Body.String())
	}

	// Status omitted entirely (incurred_on supplied so the failure is the
	// status check, not the incurred_on-required check).
	rec = do(http.MethodPut, "/api/v1/expenses/1", `{"name": "thingamajig", "amount_cents": 300, "incurred_on": "2020-05-05"}`)
	if rec.Code != 422 {
		t.Errorf("omitted status: got %d, want 422, body %s", rec.Code, rec.Body.String())
	}

	item, err := model.Find(id)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if item.Status != "bought" {
		t.Errorf("row mutated: status got %q, want unchanged %q", item.Status, "bought")
	}
	if item.IncurredOn != "2020-05-05" {
		t.Errorf("row mutated: incurred_on got %q, want unchanged %q", item.IncurredOn, "2020-05-05")
	}
	if item.AmountCents != 300 {
		t.Errorf("row mutated: amount_cents got %d, want unchanged %d", item.AmountCents, 300)
	}
}

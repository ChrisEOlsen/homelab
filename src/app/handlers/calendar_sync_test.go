package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gova/app/cache"
	"gova/app/db"
)

const calendarSyncSchema = `
CREATE TABLE clients (
    id INTEGER PRIMARY KEY, name TEXT NOT NULL, match_name TEXT NOT NULL,
    email TEXT, phone TEXT, rate_cents INTEGER NOT NULL DEFAULT 10000,
    kind TEXT NOT NULL DEFAULT 'independent', is_active INTEGER NOT NULL DEFAULT 1,
    notes TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE rate_rules (
    id INTEGER PRIMARY KEY, duration_min INTEGER NOT NULL UNIQUE,
    amount_cents INTEGER NOT NULL, label TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE training_sessions (
    id INTEGER PRIMARY KEY, uid TEXT NOT NULL UNIQUE, source TEXT NOT NULL,
    client_name TEXT NOT NULL, client_id INTEGER, service TEXT,
    session_date TEXT NOT NULL, start_at TEXT NOT NULL, end_at TEXT NOT NULL,
    duration_min INTEGER NOT NULL, amount_cents INTEGER NOT NULL DEFAULT 0,
    rate_source TEXT NOT NULL DEFAULT 'unknown', override_cents INTEGER,
    status TEXT NOT NULL DEFAULT 'scheduled', needs_review INTEGER NOT NULL DEFAULT 0,
    first_seen_at TEXT, last_seen_at TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE calendar_syncs (
    id INTEGER PRIMARY KEY, finished_at TEXT, ok INTEGER NOT NULL DEFAULT 0,
    events_seen INTEGER NOT NULL DEFAULT 0, created_count INTEGER NOT NULL DEFAULT 0,
    updated_count INTEGER NOT NULL DEFAULT 0, cancelled_count INTEGER NOT NULL DEFAULT 0,
    error TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
`

// An unreachable feed must still answer 200 with an envelope the page can read.
func TestCalendarSyncPOSTReportsFailureWithoutErroring(t *testing.T) {
	t.Setenv("CALENDAR_ICS_URL", "http://127.0.0.1:1/calendar.ics")
	d := db.OpenTest(t, calendarSyncSchema)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/calendar/sync", nil)
	rec := httptest.NewRecorder()
	CalendarSyncPOST(d.Read, d.Write, cache.New())(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v (body %s)", err, rec.Body.String())
	}
	if !env.OK {
		t.Fatal("envelope ok must be true — the endpoint worked, the feed did not")
	}
	if env.Data.OK || env.Data.Error == "" {
		t.Fatalf("the result should carry the failure: %+v", env.Data)
	}
}

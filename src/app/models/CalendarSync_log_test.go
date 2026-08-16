package models

import (
	"database/sql"
	"testing"

	"gova/app/cache"
	"gova/app/db"
)

const calendarSyncSchema = `
CREATE TABLE calendar_syncs (
    id INTEGER PRIMARY KEY,
    finished_at TEXT,
    ok INTEGER NOT NULL DEFAULT 0,
    events_seen INTEGER NOT NULL DEFAULT 0,
    created_count INTEGER NOT NULL DEFAULT 0,
    updated_count INTEGER NOT NULL DEFAULT 0,
    cancelled_count INTEGER NOT NULL DEFAULT 0,
    error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`

func TestLatestOnEmptyTableReturnsNil(t *testing.T) {
	d := db.OpenTest(t, calendarSyncSchema)
	m := NewCalendarSyncModel(d.Read, d.Write, cache.New())

	got, err := m.Latest()
	if err != nil {
		t.Fatalf("Latest on empty table: want nil error, got %v", err)
	}
	if got != nil {
		t.Fatalf("Latest on empty table: want nil result, got %+v", got)
	}
}

func TestRecordThenLatestRoundTripsSuccessfulRun(t *testing.T) {
	d := db.OpenTest(t, calendarSyncSchema)
	m := NewCalendarSyncModel(d.Read, d.Write, cache.New())

	rec := CalendarSyncRecord{
		FinishedAt: "2026-08-15 12:50:00",
		OK:         true,
		EventsSeen: 12,
		Created:    3,
		Updated:    2,
		Cancelled:  1,
	}
	if err := m.Record(rec); err != nil {
		t.Fatalf("Record: %v", err)
	}

	got, err := m.Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got == nil {
		t.Fatal("Latest: want a row, got nil")
	}
	if !got.Ok {
		t.Errorf("Ok: want true, got false")
	}
	if got.EventsSeen != 12 {
		t.Errorf("EventsSeen: want 12, got %d", got.EventsSeen)
	}
	if got.CreatedCount != 3 {
		t.Errorf("CreatedCount: want 3, got %d", got.CreatedCount)
	}
	if got.UpdatedCount != 2 {
		t.Errorf("UpdatedCount: want 2, got %d", got.UpdatedCount)
	}
	if got.CancelledCount != 1 {
		t.Errorf("CancelledCount: want 1, got %d", got.CancelledCount)
	}
	if got.FinishedAt == nil || *got.FinishedAt != "2026-08-15 12:50:00" {
		t.Errorf("FinishedAt: want 2026-08-15 12:50:00, got %v", got.FinishedAt)
	}
}

func TestRecordFailedRunStoresErrorAndEmptyErrorStoresNull(t *testing.T) {
	d := db.OpenTest(t, calendarSyncSchema)
	m := NewCalendarSyncModel(d.Read, d.Write, cache.New())

	// A failed run: OK=false, non-empty Error — the message must round-trip.
	if err := m.Record(CalendarSyncRecord{
		FinishedAt: "2026-08-15 14:20:00",
		OK:         false,
		Error:      "feed returned 500",
	}); err != nil {
		t.Fatalf("Record (failed run): %v", err)
	}

	got, err := m.Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got == nil {
		t.Fatal("Latest: want a row, got nil")
	}
	if got.Ok {
		t.Errorf("Ok: want false for a failed run, got true")
	}
	if got.Error == nil || *got.Error != "feed returned 500" {
		t.Errorf("Error: want %q, got %v", "feed returned 500", got.Error)
	}

	// A separate successful run with an empty Error must store SQL NULL, not
	// an empty string.
	if err := m.Record(CalendarSyncRecord{
		FinishedAt: "2026-08-15 15:00:00",
		OK:         true,
	}); err != nil {
		t.Fatalf("Record (empty error): %v", err)
	}

	var raw sql.NullString
	if err := d.Read.QueryRow(
		"SELECT error FROM calendar_syncs WHERE id = (SELECT MAX(id) FROM calendar_syncs)",
	).Scan(&raw); err != nil {
		t.Fatalf("querying raw error column: %v", err)
	}
	if raw.Valid {
		t.Fatalf("error column: want SQL NULL, got string %q", raw.String)
	}
}

func TestLatestReturnsSecondRunAfterTwoRecords(t *testing.T) {
	d := db.OpenTest(t, calendarSyncSchema)
	m := NewCalendarSyncModel(d.Read, d.Write, cache.New())

	if err := m.Record(CalendarSyncRecord{FinishedAt: "2026-08-15 12:50:00", OK: true, EventsSeen: 5}); err != nil {
		t.Fatalf("Record (first): %v", err)
	}
	if err := m.Record(CalendarSyncRecord{FinishedAt: "2026-08-15 14:20:00", OK: false, Error: "timeout"}); err != nil {
		t.Fatalf("Record (second): %v", err)
	}

	got, err := m.Latest()
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if got == nil {
		t.Fatal("Latest: want a row, got nil")
	}
	if got.FinishedAt == nil || *got.FinishedAt != "2026-08-15 14:20:00" {
		t.Errorf("Latest should return the second run's finished_at, got %v", got.FinishedAt)
	}
	if got.Ok {
		t.Errorf("Latest should return the second (failed) run, got Ok=true")
	}
	if got.Error == nil || *got.Error != "timeout" {
		t.Errorf("Latest should return the second run's error, got %v", got.Error)
	}
}

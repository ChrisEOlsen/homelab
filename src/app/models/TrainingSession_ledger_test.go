package models

import (
	"testing"

	"gova/app/cache"
	"gova/app/db"
)

const trainingSessionSchema = `
CREATE TABLE training_sessions (
    id INTEGER PRIMARY KEY,
    uid TEXT NOT NULL UNIQUE,
    source TEXT NOT NULL,
    client_name TEXT NOT NULL,
    client_id INTEGER,
    service TEXT,
    session_date TEXT NOT NULL,
    start_at TEXT NOT NULL,
    end_at TEXT NOT NULL,
    duration_min INTEGER NOT NULL,
    amount_cents INTEGER NOT NULL DEFAULT 0,
    rate_source TEXT NOT NULL DEFAULT 'unknown',
    override_cents INTEGER,
    status TEXT NOT NULL DEFAULT 'scheduled',
    needs_review INTEGER NOT NULL DEFAULT 0,
    first_seen_at TEXT,
    last_seen_at TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`

func newTestSession(uid, date string, cents int) TrainingSessionUpsert {
	return TrainingSessionUpsert{
		UID: uid, Source: "cc", ClientName: "John Kublacki",
		SessionDate: date, StartAt: date + " 11:00:00", EndAt: date + " 12:00:00",
		DurationMin: 60, AmountCents: cents, RateSource: "client", Status: "scheduled",
	}
}

func TestUpsertFromCalendarIsIdempotent(t *testing.T) {
	d := db.OpenTest(t, trainingSessionSchema)
	m := NewTrainingSessionModel(d.Read, d.Write, cache.New())

	created, err := m.UpsertFromCalendar(newTestSession("cc-1", "2026-08-10", 10000), "2026-08-10 12:00:00")
	if err != nil || !created {
		t.Fatalf("first upsert: created=%v err=%v", created, err)
	}
	created, err = m.UpsertFromCalendar(newTestSession("cc-1", "2026-08-10", 10000), "2026-08-10 12:10:00")
	if err != nil || created {
		t.Fatalf("second upsert should update, not create: created=%v err=%v", created, err)
	}

	var n int
	if err := d.Read.QueryRow("SELECT COUNT(*) FROM training_sessions").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 row after two syncs, got %d", n)
	}
}

func TestUpsertPreservesOverrideAndIgnore(t *testing.T) {
	d := db.OpenTest(t, trainingSessionSchema)
	m := NewTrainingSessionModel(d.Read, d.Write, cache.New())

	if _, err := m.UpsertFromCalendar(newTestSession("cc-1", "2026-08-10", 10000), "now"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Write.Exec(
		"UPDATE training_sessions SET override_cents = 7500, amount_cents = 7500, rate_source = 'override', status = 'ignored' WHERE uid = 'cc-1'",
	); err != nil {
		t.Fatal(err)
	}

	// The feed re-reports it at the default price; the manual decisions must win.
	if _, err := m.UpsertFromCalendar(newTestSession("cc-1", "2026-08-10", 10000), "now"); err != nil {
		t.Fatal(err)
	}

	var cents int
	var rateSource, status string
	if err := d.Read.QueryRow(
		"SELECT amount_cents, rate_source, status FROM training_sessions WHERE uid = 'cc-1'",
	).Scan(&cents, &rateSource, &status); err != nil {
		t.Fatal(err)
	}
	if cents != 7500 || rateSource != "override" || status != "ignored" {
		t.Fatalf("manual decisions lost: cents=%d rate_source=%s status=%s", cents, rateSource, status)
	}
}

func TestCancelMissingRespectsWindow(t *testing.T) {
	d := db.OpenTest(t, trainingSessionSchema)
	m := NewTrainingSessionModel(d.Read, d.Write, cache.New())

	// One inside the window and gone from the feed, one older than the window.
	if _, err := m.UpsertFromCalendar(newTestSession("cc-inside", "2026-08-12", 10000), "now"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.UpsertFromCalendar(newTestSession("cc-frozen", "2026-07-02", 10000), "now"); err != nil {
		t.Fatal(err)
	}

	n, err := m.CancelMissing("2026-08-10", "2026-09-14", []string{"cc-other"})
	if err != nil {
		t.Fatalf("CancelMissing: %v", err)
	}
	if n != 1 {
		t.Fatalf("want 1 cancellation, got %d", n)
	}

	var inside, frozen string
	if err := d.Read.QueryRow("SELECT status FROM training_sessions WHERE uid = 'cc-inside'").Scan(&inside); err != nil {
		t.Fatal(err)
	}
	if err := d.Read.QueryRow("SELECT status FROM training_sessions WHERE uid = 'cc-frozen'").Scan(&frozen); err != nil {
		t.Fatal(err)
	}
	if inside != "cancelled" {
		t.Fatalf("in-window missing session should be cancelled, got %q", inside)
	}
	if frozen != "scheduled" {
		t.Fatalf("out-of-window session must be frozen, got %q", frozen)
	}
}

func TestMonthIncomeSplitsEarnedFromProjected(t *testing.T) {
	d := db.OpenTest(t, trainingSessionSchema)
	m := NewTrainingSessionModel(d.Read, d.Write, cache.New())

	if _, err := m.UpsertFromCalendar(newTestSession("cc-past", "2026-08-03", 10000), "now"); err != nil {
		t.Fatal(err)
	}
	if _, err := m.UpsertFromCalendar(newTestSession("cc-future", "2026-08-28", 10000), "now"); err != nil {
		t.Fatal(err)
	}

	got, err := m.MonthIncome("2026-08", "2026-08-15 09:00:00")
	if err != nil {
		t.Fatalf("MonthIncome: %v", err)
	}
	if got.EarnedCents != 10000 {
		t.Fatalf("earned: want 10000, got %d", got.EarnedCents)
	}
	if got.ProjectedCents != 20000 {
		t.Fatalf("projected: want 20000, got %d", got.ProjectedCents)
	}
	if got.SessionCount != 2 {
		t.Fatalf("count: want 2, got %d", got.SessionCount)
	}
	if got.BySource["cc"] != 10000 {
		t.Fatalf("by_source cc: want 10000 earned, got %d", got.BySource["cc"])
	}
}

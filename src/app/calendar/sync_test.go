package calendar

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"gova/app/cache"
	"gova/app/db"
	"gova/app/models"
)

const syncSchema = `
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
INSERT INTO clients (name, match_name, rate_cents, kind) VALUES ('Ofer Rubin', 'Ofer Rubin', 10000, 'independent');
INSERT INTO rate_rules (duration_min, amount_cents) VALUES (45, 5000), (60, 6000);
`

// syncSchemaWithDurationCheck is syncSchema with one extra constraint on
// training_sessions, used only by TestRunContinuesAfterAStoreFailureAndStillReconciles
// to induce a clean, deterministic per-row store failure (a zero-duration
// event) without touching any production code.
const syncSchemaWithDurationCheck = `
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
    duration_min INTEGER NOT NULL CHECK (duration_min > 0), amount_cents INTEGER NOT NULL DEFAULT 0,
    rate_source TEXT NOT NULL DEFAULT 'unknown', override_cents INTEGER,
    status TEXT NOT NULL DEFAULT 'scheduled', needs_review INTEGER NOT NULL DEFAULT 0,
    first_seen_at TEXT, last_seen_at TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE calendar_syncs (
    id INTEGER PRIMARY KEY, finished_at TEXT, ok INTEGER NOT NULL DEFAULT 0,
    events_seen INTEGER NOT NULL DEFAULT 0, created_count INTEGER NOT NULL DEFAULT 0,
    updated_count INTEGER NOT NULL DEFAULT 0, cancelled_count INTEGER NOT NULL DEFAULT 0,
    error TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
INSERT INTO clients (name, match_name, rate_cents, kind) VALUES ('Ofer Rubin', 'Ofer Rubin', 10000, 'independent');
INSERT INTO rate_rules (duration_min, amount_cents) VALUES (45, 5000), (60, 6000);
`

func newSyncService(t *testing.T, d *db.DB, url string) *Service {
	t.Helper()
	c := cache.New()
	return NewService(url,
		models.NewTrainingSessionModel(d.Read, d.Write, c),
		models.NewClientModel(d.Read, d.Write, c),
		models.NewRateRuleModel(d.Read, d.Write, c),
		models.NewCalendarSyncModel(d.Read, d.Write, c),
	)
}

func feedServer(t *testing.T, body *string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(*body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func event(uid, summary, source, start, end string) string {
	return "BEGIN:VEVENT\r\nUID:" + uid + "\r\nSUMMARY:" + summary +
		"\r\nDTSTART;TZID=America/New_York:" + start +
		"\r\nDTEND;TZID=America/New_York:" + end +
		"\r\nX-CALSYNC-SOURCE:" + source + "\r\nEND:VEVENT\r\n"
}

func TestRunPricesAndReconciles(t *testing.T) {
	d := db.OpenTest(t, syncSchema)

	body := "BEGIN:VCALENDAR\r\n" +
		event("cc-1", "Ofer Rubin", "cc", "20260810T110000", "20260810T120000") +
		event("wl-1", "Ran Rubin — 45-Minute Training", "wl", "20260811T090000", "20260811T094500") +
		"END:VCALENDAR\r\n"
	srv := feedServer(t, &body)
	svc := newSyncService(t, d, srv.URL)

	res := svc.Run(context.Background())
	if !res.OK || res.Created != 2 {
		t.Fatalf("first run: %+v", res)
	}

	var oferCents, ranCents int
	if err := d.Read.QueryRow("SELECT amount_cents FROM training_sessions WHERE uid='cc-1'").Scan(&oferCents); err != nil {
		t.Fatal(err)
	}
	if err := d.Read.QueryRow("SELECT amount_cents FROM training_sessions WHERE uid='wl-1'").Scan(&ranCents); err != nil {
		t.Fatal(err)
	}
	if oferCents != 10000 {
		t.Fatalf("Ofer should price at $100, got %d", oferCents)
	}
	if ranCents != 5000 {
		t.Fatalf("Ran should price at the 45-minute rule, got %d", ranCents)
	}

	// Second run: the gym appointment is gone from the feed. It is inside the
	// window this feed covers, so it must be cancelled, not deleted.
	body = "BEGIN:VCALENDAR\r\n" +
		event("cc-1", "Ofer Rubin", "cc", "20260810T110000", "20260810T120000") +
		event("cc-2", "Ofer Rubin", "cc", "20260812T110000", "20260812T120000") +
		"END:VCALENDAR\r\n"

	res = svc.Run(context.Background())
	if !res.OK {
		t.Fatalf("second run failed: %+v", res)
	}
	if res.Cancelled != 1 {
		t.Fatalf("want 1 cancellation, got %d", res.Cancelled)
	}

	var status string
	if err := d.Read.QueryRow("SELECT status FROM training_sessions WHERE uid='wl-1'").Scan(&status); err != nil {
		t.Fatalf("the cancelled session must still exist: %v", err)
	}
	if status != "cancelled" {
		t.Fatalf("want cancelled, got %q", status)
	}
}

func TestRunRefusesToReconcileEmptyFeed(t *testing.T) {
	d := db.OpenTest(t, syncSchema)

	body := "BEGIN:VCALENDAR\r\n" +
		event("cc-1", "Ofer Rubin", "cc", "20260810T110000", "20260810T120000") +
		"END:VCALENDAR\r\n"
	srv := feedServer(t, &body)
	svc := newSyncService(t, d, srv.URL)

	if res := svc.Run(context.Background()); !res.OK {
		t.Fatalf("seed run: %+v", res)
	}

	body = "BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"
	res := svc.Run(context.Background())
	if res.OK {
		t.Fatal("an empty feed must be recorded as a failed run")
	}

	var status string
	if err := d.Read.QueryRow("SELECT status FROM training_sessions WHERE uid='cc-1'").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "scheduled" {
		t.Fatalf("an empty feed must not cancel anything, got %q", status)
	}
}

func TestRunRecordsFailureWhenFeedIsUnreachable(t *testing.T) {
	d := db.OpenTest(t, syncSchema)
	svc := newSyncService(t, d, "http://127.0.0.1:1/calendar.ics")

	res := svc.Run(context.Background())
	if res.OK || res.Error == "" {
		t.Fatalf("want a recorded failure, got %+v", res)
	}

	var n int
	if err := d.Read.QueryRow("SELECT COUNT(*) FROM calendar_syncs WHERE ok = 0").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 failed run logged, got %d", n)
	}
}

// TestRunNeverCancelsHistoryOutsideTheFeedWindow proves the window bound
// end-to-end: a session dated well before the earliest event the feed
// actually reported must stay 'scheduled' even though the feed never
// mentions its UID. This is the invariant that makes the ledger permanent —
// the feed only ever covers the current week plus five forward, so anything
// older than that is frozen history the feed can no longer speak about, and
// CancelMissing must never widen its window to reach it.
func TestRunNeverCancelsHistoryOutsideTheFeedWindow(t *testing.T) {
	d := db.OpenTest(t, syncSchema)

	// Seed a session dated months before the feed's earliest event. It is
	// not 'manual' — it came from a real past sync — so only the date
	// window, not the source guard, can protect it.
	if _, err := d.Write.Exec(`
INSERT INTO training_sessions
    (uid, source, client_name, session_date, start_at, end_at, duration_min,
     amount_cents, rate_source, status, first_seen_at, last_seen_at)
VALUES
    ('wl-old', 'wl', 'Ran Rubin', '2026-06-01', '2026-06-01 09:00:00',
     '2026-06-01 09:45:00', 45, 5000, 'rule', 'scheduled',
     '2026-06-01 09:00:00', '2026-06-01 09:00:00')`); err != nil {
		t.Fatalf("seed frozen-history session: %v", err)
	}

	body := "BEGIN:VCALENDAR\r\n" +
		event("cc-1", "Ofer Rubin", "cc", "20260810T110000", "20260810T120000") +
		event("wl-1", "Ran Rubin — 45-Minute Training", "wl", "20260811T090000", "20260811T094500") +
		"END:VCALENDAR\r\n"
	srv := feedServer(t, &body)
	svc := newSyncService(t, d, srv.URL)

	res := svc.Run(context.Background())
	if !res.OK {
		t.Fatalf("run: %+v", res)
	}
	if res.Cancelled != 0 {
		t.Fatalf("frozen history is outside the feed window and must not be touched, got %d cancellations", res.Cancelled)
	}

	var status string
	if err := d.Read.QueryRow("SELECT status FROM training_sessions WHERE uid='wl-old'").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "scheduled" {
		t.Fatalf("session before the feed window must stay scheduled, got %q", status)
	}
}

// TestRunClampsReconcileWindowAgainstAWildEventDate proves FIX 1: a single
// event with a wild DTSTART (an upstream year typo) must not drag the
// reconcile floor back far enough to sweep up real frozen history. The wild
// event is still upserted as data -- only the CancelMissing window is
// clamped.
func TestRunClampsReconcileWindowAgainstAWildEventDate(t *testing.T) {
	d := db.OpenTest(t, syncSchema)

	// A real session synced years ago, long before the clamp floor. Only the
	// window clamp -- not the source guard, since this is not 'manual' -- can
	// protect it from a wild-dated feed.
	if _, err := d.Write.Exec(`
INSERT INTO training_sessions
    (uid, source, client_name, session_date, start_at, end_at, duration_min,
     amount_cents, rate_source, status, first_seen_at, last_seen_at)
VALUES
    ('old-real', 'wl', 'Ran Rubin', '2016-08-10', '2016-08-10 09:00:00',
     '2016-08-10 09:45:00', 45, 5000, 'rule', 'scheduled',
     '2016-08-10 09:00:00', '2016-08-10 09:00:00')`); err != nil {
		t.Fatalf("seed frozen-history session: %v", err)
	}

	body := "BEGIN:VCALENDAR\r\n" +
		event("wild-1", "Ofer Rubin", "cc", "20160810T110000", "20160810T120000") +
		event("cc-1", "Ofer Rubin", "cc", "20260810T110000", "20260810T120000") +
		"END:VCALENDAR\r\n"
	srv := feedServer(t, &body)
	svc := newSyncService(t, d, srv.URL)

	res := svc.Run(context.Background())
	if !res.OK {
		t.Fatalf("run: %+v", res)
	}
	if res.Cancelled != 0 {
		t.Fatalf("the wild event must not widen the reconcile floor back to 2016, got %d cancellations", res.Cancelled)
	}

	var status string
	if err := d.Read.QueryRow("SELECT status FROM training_sessions WHERE uid='old-real'").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "scheduled" {
		t.Fatalf("the wild feed date must not have dragged the reconcile floor back to old-real, got %q", status)
	}

	// The wild event itself is still real data and must have been upserted.
	var wildCount int
	if err := d.Read.QueryRow("SELECT COUNT(*) FROM training_sessions WHERE uid='wild-1'").Scan(&wildCount); err != nil {
		t.Fatal(err)
	}
	if wildCount != 1 {
		t.Fatalf("the wild-dated event should still be stored as a ledger row, got %d rows", wildCount)
	}
}

// TestRunContinuesAfterAStoreFailureAndStillReconciles proves FIX 3: a
// persistent per-row store failure must not stop later events from being
// applied, and must not skip CancelMissing. The training_sessions schema here
// adds a CHECK on duration_min that a zero-duration event trips, giving a
// clean, deterministic per-row failure without touching production code.
func TestRunContinuesAfterAStoreFailureAndStillReconciles(t *testing.T) {
	d := db.OpenTest(t, syncSchemaWithDurationCheck)

	// Seed a session that is in-window and unseen by this run's feed, so we
	// can prove CancelMissing still executed despite the mid-loop failure.
	if _, err := d.Write.Exec(`
INSERT INTO training_sessions
    (uid, source, client_name, session_date, start_at, end_at, duration_min,
     amount_cents, rate_source, status, first_seen_at, last_seen_at)
VALUES
    ('stale-1', 'wl', 'Someone Else', '2026-08-10', '2026-08-10 09:00:00',
     '2026-08-10 09:45:00', 45, 5000, 'rule', 'scheduled',
     '2026-08-01 09:00:00', '2026-08-01 09:00:00')`); err != nil {
		t.Fatalf("seed stale session: %v", err)
	}

	// bad-1 has an identical DTSTART/DTEND, so DurationMin() is 0 and the
	// insert trips the CHECK trigger. cc-1 and cc-2 are ordinary events that
	// must still be applied.
	body := "BEGIN:VCALENDAR\r\n" +
		event("cc-1", "Ofer Rubin", "cc", "20260810T110000", "20260810T120000") +
		event("bad-1", "Ofer Rubin", "cc", "20260811T110000", "20260811T110000") +
		event("cc-2", "Ofer Rubin", "cc", "20260812T110000", "20260812T120000") +
		"END:VCALENDAR\r\n"
	srv := feedServer(t, &body)
	svc := newSyncService(t, d, srv.URL)

	res := svc.Run(context.Background())
	if res.OK {
		t.Fatalf("a run with a store failure must report OK: false, got %+v", res)
	}
	if res.Failed != 1 {
		t.Fatalf("want 1 failed event, got %d (%+v)", res.Failed, res)
	}
	if res.Created != 2 {
		t.Fatalf("want the other 2 events applied despite the failure, got %d created (%+v)", res.Created, res)
	}

	var n int
	if err := d.Read.QueryRow("SELECT COUNT(*) FROM training_sessions WHERE uid IN ('cc-1','cc-2')").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("the healthy events should have been stored regardless of the failure, got %d", n)
	}

	// Reconciliation must still have run: stale-1 was in-window and unseen.
	var status string
	if err := d.Read.QueryRow("SELECT status FROM training_sessions WHERE uid='stale-1'").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "cancelled" {
		t.Fatalf("reconciliation must still run after a store failure, want stale-1 cancelled, got %q", status)
	}
}

// TestRunRefusesASecondConcurrentRun proves FIX 2: runMu is package scope, so
// a run already in flight (simulated here by holding the lock directly)
// causes a second Run to return immediately with a clear failure rather than
// queueing behind it -- and that refusal is never logged as a real run.
func TestRunRefusesASecondConcurrentRun(t *testing.T) {
	d := db.OpenTest(t, syncSchema)

	body := "BEGIN:VCALENDAR\r\n" +
		event("cc-1", "Ofer Rubin", "cc", "20260810T110000", "20260810T120000") +
		"END:VCALENDAR\r\n"
	srv := feedServer(t, &body)
	svc := newSyncService(t, d, srv.URL)

	runMu.Lock()
	res := svc.Run(context.Background())
	runMu.Unlock()

	if res.OK {
		t.Fatalf("a concurrent run must report OK: false, got %+v", res)
	}
	if res.Error != "a sync is already running" {
		t.Fatalf("want the already-running error, got %q", res.Error)
	}

	var n int
	if err := d.Read.QueryRow("SELECT COUNT(*) FROM calendar_syncs").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("a refused run is not a real run and must not be logged, got %d calendar_syncs rows", n)
	}
}

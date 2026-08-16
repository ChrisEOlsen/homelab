package models

import (
	"testing"

	"gova/app/cache"
	"gova/app/db"
)

const subscriptionSchema = `
CREATE TABLE subscriptions (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    amount_cents INTEGER NOT NULL,
    cadence TEXT NOT NULL DEFAULT 'monthly',
    billing_day INTEGER,
    is_active INTEGER NOT NULL DEFAULT 1,
    started_on TEXT NOT NULL,
    ended_on TEXT,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`

func TestMonthlyEquivalentNormalizesCadence(t *testing.T) {
	if got := monthlyEquivalent("monthly", 1500); got != 1500 {
		t.Fatalf("monthly: want 1500, got %d", got)
	}
	if got := monthlyEquivalent("yearly", 12000); got != 1000 {
		t.Fatalf("yearly: want 1000, got %d", got)
	}
	if got := monthlyEquivalent("weekly", 1000); got != 4333 {
		t.Fatalf("weekly: want 4333, got %d", got)
	}
}

func TestMonthlyEquivalentForRespectsSpan(t *testing.T) {
	d := db.OpenTest(t, subscriptionSchema)
	m := NewSubscriptionModel(d.Read, d.Write, cache.New())

	if _, err := d.Write.Exec(`
INSERT INTO subscriptions (name, amount_cents, cadence, is_active, started_on, ended_on) VALUES
  ('Spotify',  1199, 'monthly', 1, '2026-01-01', NULL),
  ('Old gym',  8000, 'monthly', 0, '2025-01-01', '2026-07-15'),
  ('Future',   5000, 'monthly', 1, '2026-12-01', NULL)`); err != nil {
		t.Fatal(err)
	}

	got, err := m.MonthlyEquivalentFor("2026-08")
	if err != nil {
		t.Fatalf("MonthlyEquivalentFor: %v", err)
	}
	if got != 1199 {
		t.Fatalf("want only the live subscription (1199), got %d", got)
	}

	// The ended one is still counted in the month it was live.
	got, err = m.MonthlyEquivalentFor("2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if got != 9199 {
		t.Fatalf("july: want 9199, got %d", got)
	}
}

func TestMonthsBetween(t *testing.T) {
	if got := monthsBetween("2026-08", "2026-08"); got != 1 {
		t.Fatalf("same month: want 1, got %d", got)
	}
	if got := monthsBetween("2026-11", "2027-02"); got != 4 {
		t.Fatalf("across a year: want 4, got %d", got)
	}
	if got := monthsBetween("2026-08", "2026-07"); got != 0 {
		t.Fatalf("reversed: want 0, got %d", got)
	}
}

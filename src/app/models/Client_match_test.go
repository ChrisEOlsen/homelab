package models

import (
	"testing"

	"gova/app/cache"
	"gova/app/db"
)

const clientSchema = `
CREATE TABLE clients (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    match_name TEXT NOT NULL,
    email TEXT,
    phone TEXT,
    rate_cents INTEGER NOT NULL DEFAULT 10000,
    kind TEXT NOT NULL DEFAULT 'independent',
    is_active INTEGER NOT NULL DEFAULT 1,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`

func TestAllForMatchingSkipsInactive(t *testing.T) {
	d := db.OpenTest(t, clientSchema)
	m := NewClientModel(d.Read, d.Write, cache.New())

	if _, err := d.Write.Exec(
		`INSERT INTO clients (name, match_name, rate_cents, kind, is_active) VALUES
		 ('Ofer Rubin', 'Ofer Rubin', 10000, 'independent', 1),
		 ('Old Client', 'Old Client', 10000, 'independent', 0)`,
	); err != nil {
		t.Fatalf("seed: %v", err)
	}

	got, err := m.AllForMatching()
	if err != nil {
		t.Fatalf("AllForMatching: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 active client, got %d", len(got))
	}
	if got[0].MatchName != "Ofer Rubin" || got[0].RateCents != 10000 {
		t.Fatalf("unexpected row: %+v", got[0])
	}
}

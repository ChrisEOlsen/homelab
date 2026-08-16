package models

import (
	"testing"

	"gova/app/cache"
	"gova/app/db"
)

const expenseSchema = `
CREATE TABLE expenses (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    amount_cents INTEGER NOT NULL,
    category TEXT,
    status TEXT NOT NULL DEFAULT 'planned',
    incurred_on TEXT NOT NULL,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);`

func TestMonthTotalsSeparatesPlannedFromBought(t *testing.T) {
	d := db.OpenTest(t, expenseSchema)
	m := NewExpenseModel(d.Read, d.Write, cache.New())

	if _, err := d.Write.Exec(`
INSERT INTO expenses (name, amount_cents, status, incurred_on) VALUES
  ('Squat rack', 45000, 'bought',  '2026-08-04'),
  ('Protein',     6000, 'planned', '2026-08-09'),
  ('Last month',  9900, 'bought',  '2026-07-30')`); err != nil {
		t.Fatal(err)
	}

	bought, committed, err := m.MonthTotals("2026-08")
	if err != nil {
		t.Fatalf("MonthTotals: %v", err)
	}
	if bought != 45000 {
		t.Fatalf("bought: want 45000, got %d", bought)
	}
	if committed != 6000 {
		t.Fatalf("committed: want 6000, got %d", committed)
	}

	all, err := m.AllTimeBought()
	if err != nil {
		t.Fatal(err)
	}
	if all != 54900 {
		t.Fatalf("all-time bought: want 54900, got %d", all)
	}
}

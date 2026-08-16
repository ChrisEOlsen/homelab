package models

import (
	"testing"
	"database/sql"

	"gova/app/cache"
	"gova/app/db"
)

func TestRateRuleModel_CRUD(t *testing.T) {
	testDB := db.OpenTest(t, `CREATE TABLE rate_rules (
		id INTEGER PRIMARY KEY,
		duration_min INTEGER NOT NULL,
		amount_cents INTEGER NOT NULL,
		label TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	m := NewRateRuleModel(testDB.Read, testDB.Write, cache.New())

	labelTestVal := "test"
	id, err := m.Create(int64(1), int64(1), &labelTestVal)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == 0 {
		t.Fatal("Create returned id 0")
	}

	found, err := m.Find(id)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if found.ID != id {
		t.Errorf("Find: got ID %d, want %d", found.ID, id)
	}

	items, total, err := m.GetPage(50, 0, QueryOpts{})
	if err != nil {
		t.Fatalf("GetPage: %v", err)
	}
	if total != 1 {
		t.Errorf("GetPage: got total %d, want 1", total)
	}
	if len(items) != 1 {
		t.Fatalf("GetPage: got %d items, want 1", len(items))
	}
	if items[0].ID != id {
		t.Errorf("GetPage: got ID %d, want %d", items[0].ID, id)
	}

	if items[0].Label == nil {
		t.Errorf("GetPage: label was not recovered (nil)")
	} else if *items[0].Label != labelTestVal {
		t.Errorf("GetPage: label got %v, want %v", *items[0].Label, labelTestVal)
	}

	// An offset past the end returns an empty (non-nil) slice, not an error,
	// and the total still reflects the full table.
	page2, total2, err := m.GetPage(50, 50, QueryOpts{})
	if err != nil {
		t.Fatalf("GetPage(offset 50): %v", err)
	}
	if page2 == nil {
		t.Error("GetPage past the end returned a nil slice; want empty")
	}
	if len(page2) != 0 {
		t.Errorf("GetPage(offset 50): got %d items, want 0", len(page2))
	}
	if total2 != 1 {
		t.Errorf("GetPage(offset 50): got total %d, want 1", total2)
	}

	if err := m.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := m.Find(id); err == nil {
		t.Error("Find after Delete: expected error, got nil")
	}
}


func TestRateRuleModel_Update(t *testing.T) {
	testDB := db.OpenTest(t, `CREATE TABLE rate_rules (
		id INTEGER PRIMARY KEY,
		duration_min INTEGER NOT NULL,
		amount_cents INTEGER NOT NULL,
		label TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	m := NewRateRuleModel(testDB.Read, testDB.Write, cache.New())

	labelTestVal := "test"
	id, err := m.Create(int64(1), int64(1), &labelTestVal)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Update the existing row — should succeed.
	if err := m.Update(id, int64(1), int64(1), &labelTestVal); err != nil {
		t.Fatalf("Update: %v", err)
	}
	// Update a missing row — should report sql.ErrNoRows.
	if err := m.Update(999999, int64(1), int64(1), &labelTestVal); err != sql.ErrNoRows {
		t.Errorf("Update(missing): got %v, want sql.ErrNoRows", err)
	}
}

func TestRateRuleModel_GetPageRejectsBadSort(t *testing.T) {
	testDB := db.OpenTest(t, `CREATE TABLE rate_rules (
		id INTEGER PRIMARY KEY,
		duration_min INTEGER NOT NULL,
		amount_cents INTEGER NOT NULL,
		label TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	m := NewRateRuleModel(testDB.Read, testDB.Write, cache.New())
	if _, _, err := m.GetPage(50, 0, QueryOpts{Sort: "bogus_column"}); err != ErrInvalidQuery {
		t.Errorf("GetPage bad sort: got %v, want ErrInvalidQuery", err)
	}
}


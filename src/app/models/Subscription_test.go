package models

import (
	"testing"
	"database/sql"

	"gova/app/cache"
	"gova/app/db"
)

func TestSubscriptionModel_CRUD(t *testing.T) {
	testDB := db.OpenTest(t, `CREATE TABLE subscriptions (
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
	)`)
	m := NewSubscriptionModel(testDB.Read, testDB.Write, cache.New())

	billing_dayTestVal := int64(1)
	ended_onTestVal := "test"
	notesTestVal := "test"
	id, err := m.Create("test", int64(1), "test", &billing_dayTestVal, true, "test", &ended_onTestVal, &notesTestVal)
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

	if items[0].BillingDay == nil {
		t.Errorf("GetPage: billing_day was not recovered (nil)")
	} else if *items[0].BillingDay != billing_dayTestVal {
		t.Errorf("GetPage: billing_day got %v, want %v", *items[0].BillingDay, billing_dayTestVal)
	}

	if items[0].EndedOn == nil {
		t.Errorf("GetPage: ended_on was not recovered (nil)")
	} else if *items[0].EndedOn != ended_onTestVal {
		t.Errorf("GetPage: ended_on got %v, want %v", *items[0].EndedOn, ended_onTestVal)
	}

	if items[0].Notes == nil {
		t.Errorf("GetPage: notes was not recovered (nil)")
	} else if *items[0].Notes != notesTestVal {
		t.Errorf("GetPage: notes got %v, want %v", *items[0].Notes, notesTestVal)
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


func TestSubscriptionModel_Update(t *testing.T) {
	testDB := db.OpenTest(t, `CREATE TABLE subscriptions (
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
	)`)
	m := NewSubscriptionModel(testDB.Read, testDB.Write, cache.New())

	billing_dayTestVal := int64(1)
	ended_onTestVal := "test"
	notesTestVal := "test"
	id, err := m.Create("test", int64(1), "test", &billing_dayTestVal, true, "test", &ended_onTestVal, &notesTestVal)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Update the existing row — should succeed.
	if err := m.Update(id, "test", int64(1), "test", &billing_dayTestVal, true, "test", &ended_onTestVal, &notesTestVal); err != nil {
		t.Fatalf("Update: %v", err)
	}
	// Update a missing row — should report sql.ErrNoRows.
	if err := m.Update(999999, "test", int64(1), "test", &billing_dayTestVal, true, "test", &ended_onTestVal, &notesTestVal); err != sql.ErrNoRows {
		t.Errorf("Update(missing): got %v, want sql.ErrNoRows", err)
	}
}

func TestSubscriptionModel_GetPageRejectsBadSort(t *testing.T) {
	testDB := db.OpenTest(t, `CREATE TABLE subscriptions (
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
	)`)
	m := NewSubscriptionModel(testDB.Read, testDB.Write, cache.New())
	if _, _, err := m.GetPage(50, 0, QueryOpts{Sort: "bogus_column"}); err != ErrInvalidQuery {
		t.Errorf("GetPage bad sort: got %v, want ErrInvalidQuery", err)
	}
}


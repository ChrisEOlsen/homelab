package models

import "database/sql"

// CalendarSyncRecord is one completed sync run.
type CalendarSyncRecord struct {
	FinishedAt string
	OK         bool
	EventsSeen int
	Created    int
	Updated    int
	Cancelled  int
	Error      string
}

// Record appends a run to the log. Failures are recorded too — a page that can
// say "last sync failed at 14:20, last good run 12:50" is the whole point.
func (m *CalendarSyncModel) Record(r CalendarSyncRecord) error {
	var errVal any
	if r.Error != "" {
		errVal = r.Error
	}
	ok := 0
	if r.OK {
		ok = 1
	}
	_, err := m.writeDB.Exec(`
INSERT INTO calendar_syncs
    (finished_at, ok, events_seen, created_count, updated_count, cancelled_count, error)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.FinishedAt, ok, r.EventsSeen, r.Created, r.Updated, r.Cancelled, errVal)
	if err == nil {
		m.cache.Bust("calendar_syncs:")
	}
	return err
}

// Latest returns the most recent run, or nil when the log is empty.
func (m *CalendarSyncModel) Latest() (*CalendarSync, error) {
	row := m.readDB.QueryRow(`
SELECT id, finished_at, ok, events_seen, created_count, updated_count,
       cancelled_count, error, created_at
FROM calendar_syncs ORDER BY id DESC LIMIT 1`)

	var it CalendarSync
	err := row.Scan(&it.ID, &it.FinishedAt, &it.Ok, &it.EventsSeen, &it.CreatedCount,
		&it.UpdatedCount, &it.CancelledCount, &it.Error, &it.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &it, nil
}

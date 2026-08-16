package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
	"gova/app/cache"
)

type CalendarSync struct {
	ID        int64     `json:"id"`
	FinishedAt *string `json:"finished_at"`
	Ok bool `json:"ok"`
	EventsSeen int64 `json:"events_seen"`
	CreatedCount int64 `json:"created_count"`
	UpdatedCount int64 `json:"updated_count"`
	CancelledCount int64 `json:"cancelled_count"`
	Failed int64 `json:"failed"`
	Error *string `json:"error"`
	CreatedAt Time `json:"created_at"`
}

// calendar_syncPage is the cache payload for a single page of results — items and
// total travel together so a cache hit does not need a second COUNT query.
type calendar_syncPage struct {
	Items []CalendarSync `json:"items"`
	Total int               `json:"total"`
}

type CalendarSyncModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewCalendarSyncModel(readDB, writeDB *sql.DB, c *cache.Cache) *CalendarSyncModel {
	return &CalendarSyncModel{readDB: readDB, writeDB: writeDB, cache: c}
}

// calendar_syncAllowedColumns is the whitelist for sort/filter — the model's real
// columns. orderByClause/filterField reject anything not in this list, so the
// only column names ever placed into SQL come from here.
var calendar_syncAllowedColumns = []string{"id", "finished_at", "ok", "events_seen", "created_count", "updated_count", "cancelled_count", "error", "created_at"}

// GetPage returns one window of rows plus the total (of the filtered set).
// Callers clamp limit and offset — see handlers/paging.go. Sort/filter columns
// are validated against calendar_syncAllowedColumns; an unknown column yields
// ErrInvalidQuery (handlers map it to 422).
func (m *CalendarSyncModel) GetPage(limit, offset int, opts QueryOpts) ([]CalendarSync, int, error) {
	orderBy, err := orderByClause(opts.Sort, calendar_syncAllowedColumns)
	if err != nil {
		return nil, 0, err
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, ferr := filterField(opts.FilterField, calendar_syncAllowedColumns)
		if ferr != nil {
			return nil, 0, ferr
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("calendar_syncs:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page calendar_syncPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM calendar_syncs"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, finished_at, ok, events_seen, created_count, updated_count, cancelled_count, error, created_at FROM calendar_syncs" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// Initialized non-nil: a nil slice marshals as JSON null, which breaks
	// strictly-typed clients decoding an array.
	items := []CalendarSync{}
	for rows.Next() {
		var item CalendarSync
		var finished_atNull sql.NullString
		var errorNull sql.NullString
		if err := rows.Scan(&item.ID, &finished_atNull, &item.Ok, &item.EventsSeen, &item.CreatedCount, &item.UpdatedCount, &item.CancelledCount, &errorNull, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		if finished_atNull.Valid {
			item.FinishedAt = &finished_atNull.String
		}
		if errorNull.Valid {
			item.Error = &errorNull.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(calendar_syncPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *CalendarSyncModel) Find(id int64) (*CalendarSync, error) {
	row := m.readDB.QueryRow("SELECT id, finished_at, ok, events_seen, created_count, updated_count, cancelled_count, error, created_at FROM calendar_syncs WHERE id = ?", id)
	var item CalendarSync
	var finished_atNull sql.NullString
	var errorNull sql.NullString
	if err := row.Scan(&item.ID, &finished_atNull, &item.Ok, &item.EventsSeen, &item.CreatedCount, &item.UpdatedCount, &item.CancelledCount, &errorNull, &item.CreatedAt); err != nil {
		return nil, err
	}
	if finished_atNull.Valid {
		item.FinishedAt = &finished_atNull.String
	}
	if errorNull.Valid {
		item.Error = &errorNull.String
	}
	return &item, nil
}

func (m *CalendarSyncModel) Create(finished_at *string, ok bool, events_seen int64, created_count int64, updated_count int64, cancelled_count int64, error *string) (int64, error) {
	// A nil pointer binds to SQL NULL directly — no sql.Null* wrapper needed
	// on the insert path.
	res, err := m.writeDB.Exec(
		"INSERT INTO calendar_syncs (finished_at, ok, events_seen, created_count, updated_count, cancelled_count, error) VALUES (?, ?, ?, ?, ?, ?, ?)",
		finished_at, ok, events_seen, created_count, updated_count, cancelled_count, error,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("calendar_syncs:")
	return res.LastInsertId()
}

func (m *CalendarSyncModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM calendar_syncs WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("calendar_syncs:")
	}
	return err
}



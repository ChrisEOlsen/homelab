package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gova/app/cache"
)

type LogEntry struct {
	ID         int64  `json:"id"`
	CategoryId int64  `json:"category_id"`
	EntryData  string `json:"entry_data"`
	CreatedAt  Time   `json:"created_at"`
}

type LogEntryPage struct {
	Items []LogEntry `json:"items"`
	Total int        `json:"total"`
}

var LogEntryAllowedColumns = []string{"id", "category_id", "entry_data", "created_at"}

type LogEntryModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewLogEntryModel(readDB, writeDB *sql.DB, c *cache.Cache) *LogEntryModel {
	return &LogEntryModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *LogEntryModel) GetPage(limit, offset int, opts QueryOpts) ([]LogEntry, int, error) {
	orderBy := "ORDER BY created_at DESC"
	if opts.Sort != "" {
		ob, err := orderByClause(opts.Sort, LogEntryAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		orderBy = ob
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, err := filterField(opts.FilterField, LogEntryAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("log_entries:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page LogEntryPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM log_entries"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, category_id, entry_data, created_at FROM log_entries" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []LogEntry{}
	for rows.Next() {
		var item LogEntry
		if err := rows.Scan(&item.ID, &item.CategoryId, &item.EntryData, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(LogEntryPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *LogEntryModel) Find(id int64) (*LogEntry, error) {
	row := m.readDB.QueryRow("SELECT id, category_id, entry_data, created_at FROM log_entries WHERE id = ?", id)
	var item LogEntry
	if err := row.Scan(&item.ID, &item.CategoryId, &item.EntryData, &item.CreatedAt); err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *LogEntryModel) Create(categoryID int64, entryData string) (int64, error) {
	res, err := m.writeDB.Exec("INSERT INTO log_entries (category_id, entry_data) VALUES (?, ?)", categoryID, entryData)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("log_entries:")
	return res.LastInsertId()
}

func (m *LogEntryModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM log_entries WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("log_entries:")
	}
	return err
}

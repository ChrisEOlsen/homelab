package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type LogEntry struct {
	ID        int64     `json:"id"`
	CategoryId int64 `json:"category_id"`
	EntryData string `json:"entry_data"`
	CreatedAt time.Time `json:"created_at"`
}

type LogEntryModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewLogEntryModel(readDB, writeDB *sql.DB, c *cache.Cache) *LogEntryModel {
	return &LogEntryModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *LogEntryModel) GetAll() ([]LogEntry, error) {
	const cacheKey = "log_entries:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []LogEntry
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, category_id, entry_data, created_at FROM log_entries ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []LogEntry
	for rows.Next() {
		var item LogEntry
		if err := rows.Scan(&item.ID, &item.CategoryId, &item.EntryData, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if data, err := json.Marshal(items); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, nil
}

func (m *LogEntryModel) Find(id int64) (*LogEntry, error) {
	row := m.readDB.QueryRow("SELECT id, category_id, entry_data, created_at FROM log_entries WHERE id = ?", id)
	var item LogEntry
	err := row.Scan(&item.ID, &item.CategoryId, &item.EntryData, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *LogEntryModel) Create(category_id int64, entry_data string) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO log_entries (category_id, entry_data) VALUES (?, ?)",
		category_id, entry_data,
	)
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

func (m *LogEntryModel) GetByCategory(categoryID int64) ([]LogEntry, error) {
	rows, err := m.readDB.Query(
		"SELECT id, category_id, entry_data, created_at FROM log_entries WHERE category_id = ? ORDER BY created_at DESC",
		categoryID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []LogEntry
	for rows.Next() {
		var item LogEntry
		if err := rows.Scan(&item.ID, &item.CategoryId, &item.EntryData, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

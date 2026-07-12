package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type JournalEntry struct {
	ID        int64     `json:"id"`
	Title string `json:"title"`
	Content string `json:"content"`
	Mood string `json:"mood"`
	EntryDate string `json:"entry_date"`
	CreatedAt time.Time `json:"created_at"`
}

type JournalEntryModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewJournalEntryModel(readDB, writeDB *sql.DB, c *cache.Cache) *JournalEntryModel {
	return &JournalEntryModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *JournalEntryModel) GetAll() ([]JournalEntry, error) {
	const cacheKey = "journal_entries:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []JournalEntry
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, title, content, mood, entry_date, created_at FROM journal_entries ORDER BY entry_date DESC, created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []JournalEntry
	for rows.Next() {
		var item JournalEntry
		var title sql.NullString
		if err := rows.Scan(&item.ID, &title, &item.Content, &item.Mood, &item.EntryDate, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Title = title.String
		items = append(items, item)
	}
	if data, err := json.Marshal(items); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, nil
}

func (m *JournalEntryModel) Find(id int64) (*JournalEntry, error) {
	row := m.readDB.QueryRow("SELECT id, title, content, mood, entry_date, created_at FROM journal_entries WHERE id = ?", id)
	var item JournalEntry
	var title sql.NullString
	err := row.Scan(&item.ID, &title, &item.Content, &item.Mood, &item.EntryDate, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	item.Title = title.String
	return &item, nil
}

func (m *JournalEntryModel) Create(title string, content string, mood string, entry_date string) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO journal_entries (title, content, mood, entry_date) VALUES (?, ?, ?, ?)",
		title, content, mood, entry_date,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("journal_entries:")
	return res.LastInsertId()
}

func (m *JournalEntryModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM journal_entries WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("journal_entries:")
	}
	return err
}

func (m *JournalEntryModel) Update(id int64, title, content, mood, entryDate string) error {
	_, err := m.writeDB.Exec(
		"UPDATE journal_entries SET title = ?, content = ?, mood = ?, entry_date = ? WHERE id = ?",
		title, content, mood, entryDate, id,
	)
	if err == nil {
		m.cache.Bust("journal_entries:")
	}
	return err
}

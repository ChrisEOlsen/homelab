package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type CodexEntry struct {
	ID        int64     `json:"id"`
	Title string `json:"title"`
	Language string `json:"language"`
	Code string `json:"code"`
	Tags string `json:"tags"`
	Description string `json:"description"`
	BundleId string `json:"bundle_id"`
	CreatedAt time.Time `json:"created_at"`
}

type CodexEntryModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewCodexEntryModel(readDB, writeDB *sql.DB, c *cache.Cache) *CodexEntryModel {
	return &CodexEntryModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *CodexEntryModel) GetAll() ([]CodexEntry, error) {
	const cacheKey = "codex_entries:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []CodexEntry
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, title, language, code, tags, description, bundle_id, created_at FROM codex_entries ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []CodexEntry
	for rows.Next() {
		var item CodexEntry
		if err := rows.Scan(&item.ID, &item.Title, &item.Language, &item.Code, &item.Tags, &item.Description, &item.BundleId, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if data, err := json.Marshal(items); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, nil
}

func (m *CodexEntryModel) Find(id int64) (*CodexEntry, error) {
	row := m.readDB.QueryRow("SELECT id, title, language, code, tags, description, bundle_id, created_at FROM codex_entries WHERE id = ?", id)
	var item CodexEntry
	err := row.Scan(&item.ID, &item.Title, &item.Language, &item.Code, &item.Tags, &item.Description, &item.BundleId, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *CodexEntryModel) Create(title string, language string, code string, tags string, description string, bundle_id string) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO codex_entries (title, language, code, tags, description, bundle_id) VALUES (?, ?, ?, ?, ?, ?)",
		title, language, code, tags, description, bundle_id,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("codex_entries:")
	return res.LastInsertId()
}

func (m *CodexEntryModel) Update(id int64, title, language, code, tags, description, bundleID string) error {
	_, err := m.writeDB.Exec(
		"UPDATE codex_entries SET title = ?, language = ?, code = ?, tags = ?, description = ?, bundle_id = ? WHERE id = ?",
		title, language, code, tags, description, bundleID, id,
	)
	if err == nil {
		m.cache.Bust("codex_entries:")
	}
	return err
}

func (m *CodexEntryModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM codex_entries WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("codex_entries:")
	}
	return err
}

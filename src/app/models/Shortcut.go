package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type Shortcut struct {
	ID        int64     `json:"id"`
	Title string `json:"title"`
	Url string `json:"url"`
	CreatedAt time.Time `json:"created_at"`
}

type ShortcutModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewShortcutModel(readDB, writeDB *sql.DB, c *cache.Cache) *ShortcutModel {
	return &ShortcutModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *ShortcutModel) GetAll() ([]Shortcut, error) {
	const cacheKey = "shortcuts:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []Shortcut
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, title, url, created_at FROM shortcuts ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Shortcut
	for rows.Next() {
		var item Shortcut
		if err := rows.Scan(&item.ID, &item.Title, &item.Url, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if data, err := json.Marshal(items); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, nil
}

func (m *ShortcutModel) Find(id int64) (*Shortcut, error) {
	row := m.readDB.QueryRow("SELECT id, title, url, created_at FROM shortcuts WHERE id = ?", id)
	var item Shortcut
	err := row.Scan(&item.ID, &item.Title, &item.Url, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *ShortcutModel) Create(title string, url string) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO shortcuts (title, url) VALUES (?, ?)",
		title, url,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("shortcuts:")
	return res.LastInsertId()
}

func (m *ShortcutModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM shortcuts WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("shortcuts:")
	}
	return err
}

package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type Focus struct {
	ID        int64     `json:"id"`
	Text string `json:"text"`
	SortOrder int64 `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

type FocusModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewFocusModel(readDB, writeDB *sql.DB, c *cache.Cache) *FocusModel {
	return &FocusModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *FocusModel) GetAll() ([]Focus, error) {
	const cacheKey = "focuses:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []Focus
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, text, sort_order, created_at FROM focuses ORDER BY sort_order ASC, created_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Focus
	for rows.Next() {
		var item Focus
		if err := rows.Scan(&item.ID, &item.Text, &item.SortOrder, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if data, err := json.Marshal(items); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, nil
}

func (m *FocusModel) Find(id int64) (*Focus, error) {
	row := m.readDB.QueryRow("SELECT id, text, sort_order, created_at FROM focuses WHERE id = ?", id)
	var item Focus
	err := row.Scan(&item.ID, &item.Text, &item.SortOrder, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *FocusModel) Create(text string, sort_order int64) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO focuses (text, sort_order) VALUES (?, ?)",
		text, sort_order,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("focuses:")
	return res.LastInsertId()
}

func (m *FocusModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM focuses WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("focuses:")
	}
	return err
}

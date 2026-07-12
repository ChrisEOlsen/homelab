package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type LogCategory struct {
	ID        int64     `json:"id"`
	Title string `json:"title"`
	SchemaDef string `json:"schema_def"`
	CreatedAt time.Time `json:"created_at"`
}

type LogCategoryModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewLogCategoryModel(readDB, writeDB *sql.DB, c *cache.Cache) *LogCategoryModel {
	return &LogCategoryModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *LogCategoryModel) GetAll() ([]LogCategory, error) {
	const cacheKey = "log_categories:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []LogCategory
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, title, schema_def, created_at FROM log_categories ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []LogCategory
	for rows.Next() {
		var item LogCategory
		if err := rows.Scan(&item.ID, &item.Title, &item.SchemaDef, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if data, err := json.Marshal(items); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, nil
}

func (m *LogCategoryModel) Find(id int64) (*LogCategory, error) {
	row := m.readDB.QueryRow("SELECT id, title, schema_def, created_at FROM log_categories WHERE id = ?", id)
	var item LogCategory
	err := row.Scan(&item.ID, &item.Title, &item.SchemaDef, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *LogCategoryModel) Create(title string, schema_def string) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO log_categories (title, schema_def) VALUES (?, ?)",
		title, schema_def,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("log_categories:")
	return res.LastInsertId()
}

func (m *LogCategoryModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM log_categories WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("log_categories:")
	}
	return err
}

func (m *LogCategoryModel) Update(id int64, title, schemaDef string) error {
	_, err := m.writeDB.Exec("UPDATE log_categories SET title = ?, schema_def = ? WHERE id = ?", title, schemaDef, id)
	if err == nil {
		m.cache.Bust("log_categories:")
	}
	return err
}

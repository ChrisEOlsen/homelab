package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type BookmarkCategory struct {
	ID        int64     `json:"id"`
	Title string `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

type BookmarkCategoryModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewBookmarkCategoryModel(readDB, writeDB *sql.DB, c *cache.Cache) *BookmarkCategoryModel {
	return &BookmarkCategoryModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *BookmarkCategoryModel) GetAll() ([]BookmarkCategory, error) {
	const cacheKey = "bookmark_categories:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []BookmarkCategory
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, title, created_at FROM bookmark_categories ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []BookmarkCategory
	for rows.Next() {
		var item BookmarkCategory
		if err := rows.Scan(&item.ID, &item.Title, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if data, err := json.Marshal(items); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, nil
}

func (m *BookmarkCategoryModel) Find(id int64) (*BookmarkCategory, error) {
	row := m.readDB.QueryRow("SELECT id, title, created_at FROM bookmark_categories WHERE id = ?", id)
	var item BookmarkCategory
	err := row.Scan(&item.ID, &item.Title, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *BookmarkCategoryModel) Create(title string) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO bookmark_categories (title) VALUES (?)",
		title,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("bookmark_categories:")
	return res.LastInsertId()
}

func (m *BookmarkCategoryModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM bookmark_categories WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("bookmark_categories:")
	}
	return err
}

func (m *BookmarkCategoryModel) Update(id int64, title string) error {
	_, err := m.writeDB.Exec("UPDATE bookmark_categories SET title = ? WHERE id = ?", title, id)
	if err == nil {
		m.cache.Bust("bookmark_categories:")
	}
	return err
}

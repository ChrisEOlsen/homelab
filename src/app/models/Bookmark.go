package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type Bookmark struct {
	ID        int64     `json:"id"`
	CategoryId int64 `json:"category_id"`
	Title string `json:"title"`
	Url string `json:"url"`
	Description string `json:"description"`
	CreatedAt time.Time `json:"created_at"`
}

type BookmarkModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewBookmarkModel(readDB, writeDB *sql.DB, c *cache.Cache) *BookmarkModel {
	return &BookmarkModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *BookmarkModel) GetAll() ([]Bookmark, error) {
	const cacheKey = "bookmarks:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []Bookmark
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, category_id, title, url, description, created_at FROM bookmarks ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Bookmark
	for rows.Next() {
		var item Bookmark
		if err := rows.Scan(&item.ID, &item.CategoryId, &item.Title, &item.Url, &item.Description, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if data, err := json.Marshal(items); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, nil
}

func (m *BookmarkModel) Find(id int64) (*Bookmark, error) {
	row := m.readDB.QueryRow("SELECT id, category_id, title, url, description, created_at FROM bookmarks WHERE id = ?", id)
	var item Bookmark
	err := row.Scan(&item.ID, &item.CategoryId, &item.Title, &item.Url, &item.Description, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *BookmarkModel) Create(category_id int64, title string, url string, description string) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO bookmarks (category_id, title, url, description) VALUES (?, ?, ?, ?)",
		category_id, title, url, description,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("bookmarks:")
	return res.LastInsertId()
}

func (m *BookmarkModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM bookmarks WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("bookmarks:")
	}
	return err
}

func (m *BookmarkModel) GetByCategory(categoryID int64) ([]Bookmark, error) {
	rows, err := m.readDB.Query(
		"SELECT id, category_id, title, url, description, created_at FROM bookmarks WHERE category_id = ? ORDER BY created_at DESC",
		categoryID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Bookmark
	for rows.Next() {
		var item Bookmark
		if err := rows.Scan(&item.ID, &item.CategoryId, &item.Title, &item.Url, &item.Description, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (m *BookmarkModel) Update(id, categoryID int64, title, url, description string) error {
	_, err := m.writeDB.Exec(
		"UPDATE bookmarks SET category_id = ?, title = ?, url = ?, description = ? WHERE id = ?",
		categoryID, title, url, description, id,
	)
	if err == nil {
		m.cache.Bust("bookmarks:")
	}
	return err
}

package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type VisionCategory struct {
	ID        int64     `json:"id"`
	Title string `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

type VisionCategoryModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewVisionCategoryModel(readDB, writeDB *sql.DB, c *cache.Cache) *VisionCategoryModel {
	return &VisionCategoryModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *VisionCategoryModel) GetAll() ([]VisionCategory, error) {
	const cacheKey = "vision_categories:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []VisionCategory
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, title, created_at FROM vision_categories ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []VisionCategory
	for rows.Next() {
		var item VisionCategory
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

func (m *VisionCategoryModel) Find(id int64) (*VisionCategory, error) {
	row := m.readDB.QueryRow("SELECT id, title, created_at FROM vision_categories WHERE id = ?", id)
	var item VisionCategory
	err := row.Scan(&item.ID, &item.Title, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *VisionCategoryModel) Create(title string) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO vision_categories (title) VALUES (?)",
		title,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("vision_categories:")
	return res.LastInsertId()
}

func (m *VisionCategoryModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM vision_categories WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("vision_categories:")
	}
	return err
}

package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type VisionGoal struct {
	ID        int64     `json:"id"`
	CategoryId int64 `json:"category_id"`
	Title string `json:"title"`
	TargetYear int64 `json:"target_year"`
	CreatedAt time.Time `json:"created_at"`
}

type VisionGoalModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewVisionGoalModel(readDB, writeDB *sql.DB, c *cache.Cache) *VisionGoalModel {
	return &VisionGoalModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *VisionGoalModel) GetAll() ([]VisionGoal, error) {
	const cacheKey = "vision_goals:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []VisionGoal
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, category_id, title, target_year, created_at FROM vision_goals ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []VisionGoal
	for rows.Next() {
		var item VisionGoal
		if err := rows.Scan(&item.ID, &item.CategoryId, &item.Title, &item.TargetYear, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if data, err := json.Marshal(items); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, nil
}

func (m *VisionGoalModel) Find(id int64) (*VisionGoal, error) {
	row := m.readDB.QueryRow("SELECT id, category_id, title, target_year, created_at FROM vision_goals WHERE id = ?", id)
	var item VisionGoal
	err := row.Scan(&item.ID, &item.CategoryId, &item.Title, &item.TargetYear, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *VisionGoalModel) Create(category_id int64, title string, target_year int64) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO vision_goals (category_id, title, target_year) VALUES (?, ?, ?)",
		category_id, title, target_year,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("vision_goals:")
	return res.LastInsertId()
}

func (m *VisionGoalModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM vision_goals WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("vision_goals:")
	}
	return err
}

func (m *VisionGoalModel) GetByCategory(categoryID int64) ([]VisionGoal, error) {
	rows, err := m.readDB.Query(
		"SELECT id, category_id, title, target_year, created_at FROM vision_goals WHERE category_id = ? ORDER BY target_year ASC, created_at DESC",
		categoryID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []VisionGoal
	for rows.Next() {
		var item VisionGoal
		if err := rows.Scan(&item.ID, &item.CategoryId, &item.Title, &item.TargetYear, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

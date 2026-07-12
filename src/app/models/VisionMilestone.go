package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type VisionMilestone struct {
	ID        int64     `json:"id"`
	GoalId int64 `json:"goal_id"`
	Title string `json:"title"`
	IsDone bool `json:"is_done"`
	CreatedAt time.Time `json:"created_at"`
}

type VisionMilestoneModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewVisionMilestoneModel(readDB, writeDB *sql.DB, c *cache.Cache) *VisionMilestoneModel {
	return &VisionMilestoneModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *VisionMilestoneModel) GetAll() ([]VisionMilestone, error) {
	const cacheKey = "vision_milestones:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []VisionMilestone
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, goal_id, title, is_done, created_at FROM vision_milestones ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []VisionMilestone
	for rows.Next() {
		var item VisionMilestone
		if err := rows.Scan(&item.ID, &item.GoalId, &item.Title, &item.IsDone, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if data, err := json.Marshal(items); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, nil
}

func (m *VisionMilestoneModel) Find(id int64) (*VisionMilestone, error) {
	row := m.readDB.QueryRow("SELECT id, goal_id, title, is_done, created_at FROM vision_milestones WHERE id = ?", id)
	var item VisionMilestone
	err := row.Scan(&item.ID, &item.GoalId, &item.Title, &item.IsDone, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *VisionMilestoneModel) Create(goal_id int64, title string, is_done bool) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO vision_milestones (goal_id, title, is_done) VALUES (?, ?, ?)",
		goal_id, title, is_done,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("vision_milestones:")
	return res.LastInsertId()
}

func (m *VisionMilestoneModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM vision_milestones WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("vision_milestones:")
	}
	return err
}

func (m *VisionMilestoneModel) GetByGoal(goalID int64) ([]VisionMilestone, error) {
	rows, err := m.readDB.Query(
		"SELECT id, goal_id, title, is_done, created_at FROM vision_milestones WHERE goal_id = ? ORDER BY is_done ASC, created_at ASC",
		goalID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []VisionMilestone
	for rows.Next() {
		var item VisionMilestone
		if err := rows.Scan(&item.ID, &item.GoalId, &item.Title, &item.IsDone, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (m *VisionMilestoneModel) Toggle(id int64) error {
	_, err := m.writeDB.Exec("UPDATE vision_milestones SET is_done = NOT is_done WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("vision_milestones:")
	}
	return err
}

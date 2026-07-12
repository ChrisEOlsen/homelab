package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type Subtask struct {
	ID        int64     `json:"id"`
	TodoId int64 `json:"todo_id"`
	Title string `json:"title"`
	IsDone bool `json:"is_done"`
	Description string `json:"description"`
	CreatedAt time.Time `json:"created_at"`
}

type SubtaskModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewSubtaskModel(readDB, writeDB *sql.DB, c *cache.Cache) *SubtaskModel {
	return &SubtaskModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *SubtaskModel) GetAll() ([]Subtask, error) {
	const cacheKey = "subtasks:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []Subtask
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, todo_id, title, is_done, description, created_at FROM subtasks ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Subtask
	for rows.Next() {
		var item Subtask
		var description sql.NullString
		if err := rows.Scan(&item.ID, &item.TodoId, &item.Title, &item.IsDone, &description, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Description = description.String
		items = append(items, item)
	}
	if data, err := json.Marshal(items); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, nil
}

func (m *SubtaskModel) Find(id int64) (*Subtask, error) {
	row := m.readDB.QueryRow("SELECT id, todo_id, title, is_done, description, created_at FROM subtasks WHERE id = ?", id)
	var item Subtask
	var description sql.NullString
	err := row.Scan(&item.ID, &item.TodoId, &item.Title, &item.IsDone, &description, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	item.Description = description.String
	return &item, nil
}

func (m *SubtaskModel) Create(todo_id int64, title string, is_done bool, description string) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO subtasks (todo_id, title, is_done, description) VALUES (?, ?, ?, ?)",
		todo_id, title, is_done, description,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("subtasks:")
	// The bulk todos list embeds a per-todo subtask count, so a new subtask
	// must also invalidate that cache or the count goes stale for up to 5min.
	m.cache.Bust("todos:")
	return res.LastInsertId()
}

func (m *SubtaskModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM subtasks WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("subtasks:")
		m.cache.Bust("todos:")
	}
	return err
}

func (m *SubtaskModel) GetByTodo(todoID int64) ([]Subtask, error) {
	rows, err := m.readDB.Query(
		"SELECT id, todo_id, title, is_done, description, created_at FROM subtasks WHERE todo_id = ? ORDER BY created_at ASC",
		todoID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Subtask
	for rows.Next() {
		var item Subtask
		var description sql.NullString
		if err := rows.Scan(&item.ID, &item.TodoId, &item.Title, &item.IsDone, &description, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Description = description.String
		items = append(items, item)
	}
	return items, nil
}

func (m *SubtaskModel) Update(id int64, title string) error {
	_, err := m.writeDB.Exec("UPDATE subtasks SET title = ? WHERE id = ?", title, id)
	if err == nil {
		m.cache.Bust("subtasks:")
		m.cache.Bust("todos:")
	}
	return err
}

func (m *SubtaskModel) Toggle(id int64) error {
	_, err := m.writeDB.Exec("UPDATE subtasks SET is_done = NOT is_done WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("subtasks:")
	}
	return err
}

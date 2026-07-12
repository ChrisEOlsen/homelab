package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type Todo struct {
	ID        int64     `json:"id"`
	ListId int64 `json:"list_id"`
	Title string `json:"title"`
	IsDone bool `json:"is_done"`
	Description string `json:"description"`
	SortOrder int64 `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

type TodoModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewTodoModel(readDB, writeDB *sql.DB, c *cache.Cache) *TodoModel {
	return &TodoModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *TodoModel) GetAll() ([]Todo, error) {
	const cacheKey = "todos:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []Todo
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, list_id, title, is_done, description, sort_order, created_at FROM todos ORDER BY sort_order ASC, created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Todo
	for rows.Next() {
		var item Todo
		if err := rows.Scan(&item.ID, &item.ListId, &item.Title, &item.IsDone, &item.Description, &item.SortOrder, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if data, err := json.Marshal(items); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, nil
}

func (m *TodoModel) Find(id int64) (*Todo, error) {
	row := m.readDB.QueryRow("SELECT id, list_id, title, is_done, description, sort_order, created_at FROM todos WHERE id = ?", id)
	var item Todo
	err := row.Scan(&item.ID, &item.ListId, &item.Title, &item.IsDone, &item.Description, &item.SortOrder, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *TodoModel) Create(list_id int64, title string, is_done bool, description string, sort_order int64) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO todos (list_id, title, is_done, description, sort_order) VALUES (?, ?, ?, ?, ?)",
		list_id, title, is_done, description, sort_order,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("todos:")
	return res.LastInsertId()
}

func (m *TodoModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM todos WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("todos:")
	}
	return err
}

func (m *TodoModel) GetByList(listID int64) ([]Todo, error) {
	rows, err := m.readDB.Query(
		"SELECT id, list_id, title, is_done, description, sort_order, created_at FROM todos WHERE list_id = ? ORDER BY sort_order ASC, created_at DESC",
		listID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Todo
	for rows.Next() {
		var item Todo
		if err := rows.Scan(&item.ID, &item.ListId, &item.Title, &item.IsDone, &item.Description, &item.SortOrder, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (m *TodoModel) Update(id, listID int64, title string, isDone bool, description string, sortOrder int) error {
	_, err := m.writeDB.Exec(
		"UPDATE todos SET list_id = ?, title = ?, is_done = ?, description = ?, sort_order = ? WHERE id = ?",
		listID, title, isDone, description, sortOrder, id,
	)
	if err == nil {
		m.cache.Bust("todos:")
	}
	return err
}

func (m *TodoModel) Toggle(id int64) error {
	_, err := m.writeDB.Exec("UPDATE todos SET is_done = NOT is_done WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("todos:")
	}
	return err
}

func (m *TodoModel) UpdateSortOrder(id int64, sortOrder int) error {
	_, err := m.writeDB.Exec("UPDATE todos SET sort_order = ? WHERE id = ?", sortOrder, id)
	if err == nil {
		m.cache.Bust("todos:")
	}
	return err
}

func (m *TodoModel) ClearCompleted(listID int64) error {
	_, err := m.writeDB.Exec("DELETE FROM todos WHERE list_id = ? AND is_done = 1", listID)
	if err == nil {
		m.cache.Bust("todos:")
	}
	return err
}

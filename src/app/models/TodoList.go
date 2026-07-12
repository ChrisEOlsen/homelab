package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type TodoList struct {
	ID        int64     `json:"id"`
	Title string `json:"title"`
	SortOrder int64 `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

type TodoListModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewTodoListModel(readDB, writeDB *sql.DB, c *cache.Cache) *TodoListModel {
	return &TodoListModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *TodoListModel) GetAll() ([]TodoList, error) {
	const cacheKey = "todo_lists:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []TodoList
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, title, sort_order, created_at FROM todo_lists ORDER BY sort_order ASC, created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []TodoList
	for rows.Next() {
		var item TodoList
		if err := rows.Scan(&item.ID, &item.Title, &item.SortOrder, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if data, err := json.Marshal(items); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, nil
}

func (m *TodoListModel) Find(id int64) (*TodoList, error) {
	row := m.readDB.QueryRow("SELECT id, title, sort_order, created_at FROM todo_lists WHERE id = ?", id)
	var item TodoList
	err := row.Scan(&item.ID, &item.Title, &item.SortOrder, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *TodoListModel) Create(title string, sort_order int64) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO todo_lists (title, sort_order) VALUES (?, ?)",
		title, sort_order,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("todo_lists:")
	return res.LastInsertId()
}

func (m *TodoListModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM todo_lists WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("todo_lists:")
		// Deleting a list cascades (ON DELETE CASCADE) to its todos in the DB,
		// so the todos cache must be busted too or stale entries linger until TTL.
		m.cache.Bust("todos:")
	}
	return err
}

func (m *TodoListModel) Update(id int64, title string, sortOrder int) error {
	_, err := m.writeDB.Exec("UPDATE todo_lists SET title = ?, sort_order = ? WHERE id = ?", title, sortOrder, id)
	if err == nil {
		m.cache.Bust("todo_lists:")
	}
	return err
}

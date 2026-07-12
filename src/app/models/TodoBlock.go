package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type TodoBlock struct {
	ID        int64     `json:"id"`
	TodoId int64 `json:"todo_id"`
	Header string `json:"header"`
	Content string `json:"content"`
	SortOrder int64 `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

type TodoBlockModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewTodoBlockModel(readDB, writeDB *sql.DB, c *cache.Cache) *TodoBlockModel {
	return &TodoBlockModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *TodoBlockModel) GetAll() ([]TodoBlock, error) {
	const cacheKey = "todo_blocks:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []TodoBlock
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, todo_id, header, content, sort_order, created_at FROM todo_blocks ORDER BY sort_order ASC, created_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []TodoBlock
	for rows.Next() {
		var item TodoBlock
		if err := rows.Scan(&item.ID, &item.TodoId, &item.Header, &item.Content, &item.SortOrder, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if data, err := json.Marshal(items); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, nil
}

func (m *TodoBlockModel) Find(id int64) (*TodoBlock, error) {
	row := m.readDB.QueryRow("SELECT id, todo_id, header, content, sort_order, created_at FROM todo_blocks WHERE id = ?", id)
	var item TodoBlock
	err := row.Scan(&item.ID, &item.TodoId, &item.Header, &item.Content, &item.SortOrder, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *TodoBlockModel) Create(todo_id int64, header string, content string, sort_order int64) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO todo_blocks (todo_id, header, content, sort_order) VALUES (?, ?, ?, ?)",
		todo_id, header, content, sort_order,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("todo_blocks:")
	return res.LastInsertId()
}

func (m *TodoBlockModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM todo_blocks WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("todo_blocks:")
	}
	return err
}

func (m *TodoBlockModel) GetByTodo(todoID int64) ([]TodoBlock, error) {
	rows, err := m.readDB.Query(
		"SELECT id, todo_id, header, content, sort_order, created_at FROM todo_blocks WHERE todo_id = ? ORDER BY sort_order ASC, created_at ASC",
		todoID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []TodoBlock
	for rows.Next() {
		var item TodoBlock
		if err := rows.Scan(&item.ID, &item.TodoId, &item.Header, &item.Content, &item.SortOrder, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (m *TodoBlockModel) Update(id int64, header, content string, sortOrder int64) error {
	_, err := m.writeDB.Exec(
		"UPDATE todo_blocks SET header = ?, content = ?, sort_order = ? WHERE id = ?",
		header, content, sortOrder, id,
	)
	if err == nil {
		m.cache.Bust("todo_blocks:")
	}
	return err
}

package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gova/app/cache"
)

type TodoList struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	SortOrder int64  `json:"sort_order"`
	CreatedAt Time   `json:"created_at"`
}

type TodoListPage struct {
	Items []TodoList `json:"items"`
	Total int        `json:"total"`
}

var TodoListAllowedColumns = []string{"id", "title", "sort_order", "created_at"}

type TodoListModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewTodoListModel(readDB, writeDB *sql.DB, c *cache.Cache) *TodoListModel {
	return &TodoListModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *TodoListModel) GetPage(limit, offset int, opts QueryOpts) ([]TodoList, int, error) {
	orderBy := "ORDER BY sort_order ASC, created_at DESC"
	if opts.Sort != "" {
		ob, err := orderByClause(opts.Sort, TodoListAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		orderBy = ob
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, err := filterField(opts.FilterField, TodoListAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("todo_lists:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page TodoListPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM todo_lists"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, title, sort_order, created_at FROM todo_lists" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []TodoList{}
	for rows.Next() {
		var item TodoList
		if err := rows.Scan(&item.ID, &item.Title, &item.SortOrder, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(TodoListPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *TodoListModel) Find(id int64) (*TodoList, error) {
	row := m.readDB.QueryRow("SELECT id, title, sort_order, created_at FROM todo_lists WHERE id = ?", id)
	var item TodoList
	if err := row.Scan(&item.ID, &item.Title, &item.SortOrder, &item.CreatedAt); err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *TodoListModel) Create(title string, sortOrder int64) (int64, error) {
	res, err := m.writeDB.Exec("INSERT INTO todo_lists (title, sort_order) VALUES (?, ?)", title, sortOrder)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("todo_lists:")
	return res.LastInsertId()
}

func (m *TodoListModel) Update(id int64, title string, sortOrder int) error {
	res, err := m.writeDB.Exec("UPDATE todo_lists SET title = ?, sort_order = ? WHERE id = ?", title, sortOrder, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	m.cache.Bust("todo_lists:")
	return nil
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

package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gova/app/cache"
)

type TodoBlock struct {
	ID        int64  `json:"id"`
	TodoId    int64  `json:"todo_id"`
	Header    string `json:"header"`
	Content   string `json:"content"`
	SortOrder int64  `json:"sort_order"`
	CreatedAt Time   `json:"created_at"`
}

type TodoBlockPage struct {
	Items []TodoBlock `json:"items"`
	Total int         `json:"total"`
}

var TodoBlockAllowedColumns = []string{"id", "todo_id", "header", "content", "sort_order", "created_at"}

type TodoBlockModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewTodoBlockModel(readDB, writeDB *sql.DB, c *cache.Cache) *TodoBlockModel {
	return &TodoBlockModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *TodoBlockModel) GetPage(limit, offset int, opts QueryOpts) ([]TodoBlock, int, error) {
	orderBy := "ORDER BY sort_order ASC, created_at ASC"
	if opts.Sort != "" {
		ob, err := orderByClause(opts.Sort, TodoBlockAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		orderBy = ob
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, err := filterField(opts.FilterField, TodoBlockAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("todo_blocks:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page TodoBlockPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM todo_blocks"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, todo_id, header, content, sort_order, created_at FROM todo_blocks" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []TodoBlock{}
	for rows.Next() {
		var item TodoBlock
		if err := rows.Scan(&item.ID, &item.TodoId, &item.Header, &item.Content, &item.SortOrder, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(TodoBlockPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *TodoBlockModel) Find(id int64) (*TodoBlock, error) {
	row := m.readDB.QueryRow("SELECT id, todo_id, header, content, sort_order, created_at FROM todo_blocks WHERE id = ?", id)
	var item TodoBlock
	if err := row.Scan(&item.ID, &item.TodoId, &item.Header, &item.Content, &item.SortOrder, &item.CreatedAt); err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *TodoBlockModel) Create(todoID int64, header, content string, sortOrder int64) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO todo_blocks (todo_id, header, content, sort_order) VALUES (?, ?, ?, ?)",
		todoID, header, content, sortOrder,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("todo_blocks:")
	return res.LastInsertId()
}

func (m *TodoBlockModel) Update(id int64, header, content string, sortOrder int64) error {
	res, err := m.writeDB.Exec(
		"UPDATE todo_blocks SET header = ?, content = ?, sort_order = ? WHERE id = ?",
		header, content, sortOrder, id,
	)
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
	m.cache.Bust("todo_blocks:")
	return nil
}

func (m *TodoBlockModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM todo_blocks WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("todo_blocks:")
	}
	return err
}

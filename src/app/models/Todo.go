package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gova/app/cache"
)

type Todo struct {
	ID           int64   `json:"id"`
	ListId       int64   `json:"list_id"`
	Title        string  `json:"title"`
	IsDone       bool    `json:"is_done"`
	Description  *string `json:"description"`
	SortOrder    int64   `json:"sort_order"`
	CreatedAt    Time    `json:"created_at"`
	SubtaskCount int64   `json:"subtask_count"`
}

type TodoPage struct {
	Items []Todo `json:"items"`
	Total int    `json:"total"`
}

var TodoAllowedColumns = []string{"id", "list_id", "title", "is_done", "description", "sort_order", "created_at"}

// subtaskCountExpr is the correlated subquery that embeds each todo's subtask
// count in list/detail reads, so the client never needs a second round trip
// just to show "3 subtasks".
const subtaskCountExpr = "(SELECT COUNT(*) FROM subtasks WHERE subtasks.todo_id = todos.id) AS subtask_count"

type TodoModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewTodoModel(readDB, writeDB *sql.DB, c *cache.Cache) *TodoModel {
	return &TodoModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *TodoModel) GetPage(limit, offset int, opts QueryOpts) ([]Todo, int, error) {
	orderBy := "ORDER BY sort_order ASC, created_at DESC"
	if opts.Sort != "" {
		ob, err := orderByClause(opts.Sort, TodoAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		orderBy = ob
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, err := filterField(opts.FilterField, TodoAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("todos:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page TodoPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM todos"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, list_id, title, is_done, description, sort_order, created_at, " + subtaskCountExpr +
		" FROM todos" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []Todo{}
	for rows.Next() {
		var item Todo
		var descriptionNull sql.NullString
		if err := rows.Scan(&item.ID, &item.ListId, &item.Title, &item.IsDone, &descriptionNull, &item.SortOrder, &item.CreatedAt, &item.SubtaskCount); err != nil {
			return nil, 0, err
		}
		if descriptionNull.Valid {
			item.Description = &descriptionNull.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(TodoPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *TodoModel) Find(id int64) (*Todo, error) {
	row := m.readDB.QueryRow(
		"SELECT id, list_id, title, is_done, description, sort_order, created_at, "+subtaskCountExpr+" FROM todos WHERE id = ?",
		id,
	)
	var item Todo
	var descriptionNull sql.NullString
	if err := row.Scan(&item.ID, &item.ListId, &item.Title, &item.IsDone, &descriptionNull, &item.SortOrder, &item.CreatedAt, &item.SubtaskCount); err != nil {
		return nil, err
	}
	if descriptionNull.Valid {
		item.Description = &descriptionNull.String
	}
	return &item, nil
}

func (m *TodoModel) Create(listID int64, title string, isDone bool, description *string, sortOrder int64) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO todos (list_id, title, is_done, description, sort_order) VALUES (?, ?, ?, ?, ?)",
		listID, title, isDone, description, sortOrder,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("todos:")
	return res.LastInsertId()
}

func (m *TodoModel) Update(id, listID int64, title string, isDone bool, description *string, sortOrder int) error {
	res, err := m.writeDB.Exec(
		"UPDATE todos SET list_id = ?, title = ?, is_done = ?, description = ?, sort_order = ? WHERE id = ?",
		listID, title, isDone, description, sortOrder, id,
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
	m.cache.Bust("todos:")
	return nil
}

func (m *TodoModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM todos WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("todos:")
	}
	return err
}

func (m *TodoModel) Toggle(id int64) error {
	res, err := m.writeDB.Exec("UPDATE todos SET is_done = NOT is_done WHERE id = ?", id)
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
	m.cache.Bust("todos:")
	return nil
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

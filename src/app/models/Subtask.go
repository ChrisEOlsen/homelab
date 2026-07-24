package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gova/app/cache"
)

type Subtask struct {
	ID          int64   `json:"id"`
	TodoId      int64   `json:"todo_id"`
	Title       string  `json:"title"`
	IsDone      bool    `json:"is_done"`
	Description *string `json:"description"`
	CreatedAt   Time    `json:"created_at"`
}

type SubtaskPage struct {
	Items []Subtask `json:"items"`
	Total int       `json:"total"`
}

var SubtaskAllowedColumns = []string{"id", "todo_id", "title", "is_done", "description", "created_at"}

type SubtaskModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewSubtaskModel(readDB, writeDB *sql.DB, c *cache.Cache) *SubtaskModel {
	return &SubtaskModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *SubtaskModel) GetPage(limit, offset int, opts QueryOpts) ([]Subtask, int, error) {
	orderBy := "ORDER BY created_at ASC"
	if opts.Sort != "" {
		ob, err := orderByClause(opts.Sort, SubtaskAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		orderBy = ob
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, err := filterField(opts.FilterField, SubtaskAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("subtasks:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page SubtaskPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM subtasks"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, todo_id, title, is_done, description, created_at FROM subtasks" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []Subtask{}
	for rows.Next() {
		var item Subtask
		var descriptionNull sql.NullString
		if err := rows.Scan(&item.ID, &item.TodoId, &item.Title, &item.IsDone, &descriptionNull, &item.CreatedAt); err != nil {
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

	if data, err := json.Marshal(SubtaskPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *SubtaskModel) Find(id int64) (*Subtask, error) {
	row := m.readDB.QueryRow("SELECT id, todo_id, title, is_done, description, created_at FROM subtasks WHERE id = ?", id)
	var item Subtask
	var descriptionNull sql.NullString
	if err := row.Scan(&item.ID, &item.TodoId, &item.Title, &item.IsDone, &descriptionNull, &item.CreatedAt); err != nil {
		return nil, err
	}
	if descriptionNull.Valid {
		item.Description = &descriptionNull.String
	}
	return &item, nil
}

func (m *SubtaskModel) Create(todoID int64, title string, isDone bool, description *string) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO subtasks (todo_id, title, is_done, description) VALUES (?, ?, ?, ?)",
		todoID, title, isDone, description,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("subtasks:")
	// The todos list embeds a per-todo subtask count, so a new subtask must
	// also invalidate that cache or the count goes stale for up to 5min.
	m.cache.Bust("todos:")
	return res.LastInsertId()
}

// Update changes only the subtask title — the web UI edits title inline and
// toggles done-state through Toggle, so a full-row update is intentionally not
// exposed here.
func (m *SubtaskModel) Update(id int64, title string) error {
	res, err := m.writeDB.Exec("UPDATE subtasks SET title = ? WHERE id = ?", title, id)
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
	m.cache.Bust("subtasks:")
	m.cache.Bust("todos:")
	return nil
}

func (m *SubtaskModel) Toggle(id int64) error {
	res, err := m.writeDB.Exec("UPDATE subtasks SET is_done = NOT is_done WHERE id = ?", id)
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
	m.cache.Bust("subtasks:")
	return nil
}

func (m *SubtaskModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM subtasks WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("subtasks:")
		m.cache.Bust("todos:")
	}
	return err
}

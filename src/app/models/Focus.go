package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gova/app/cache"
)

type Focus struct {
	ID        int64  `json:"id"`
	Text      string `json:"text"`
	SortOrder int64  `json:"sort_order"`
	CreatedAt Time   `json:"created_at"`
}

type FocusPage struct {
	Items []Focus `json:"items"`
	Total int     `json:"total"`
}

var FocusAllowedColumns = []string{"id", "text", "sort_order", "created_at"}

type FocusModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewFocusModel(readDB, writeDB *sql.DB, c *cache.Cache) *FocusModel {
	return &FocusModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *FocusModel) GetPage(limit, offset int, opts QueryOpts) ([]Focus, int, error) {
	orderBy := "ORDER BY sort_order ASC, created_at ASC"
	if opts.Sort != "" {
		ob, err := orderByClause(opts.Sort, FocusAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		orderBy = ob
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, err := filterField(opts.FilterField, FocusAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("focuses:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page FocusPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM focuses"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, text, sort_order, created_at FROM focuses" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []Focus{}
	for rows.Next() {
		var item Focus
		if err := rows.Scan(&item.ID, &item.Text, &item.SortOrder, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(FocusPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *FocusModel) Find(id int64) (*Focus, error) {
	row := m.readDB.QueryRow("SELECT id, text, sort_order, created_at FROM focuses WHERE id = ?", id)
	var item Focus
	if err := row.Scan(&item.ID, &item.Text, &item.SortOrder, &item.CreatedAt); err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *FocusModel) Create(text string, sortOrder int64) (int64, error) {
	res, err := m.writeDB.Exec("INSERT INTO focuses (text, sort_order) VALUES (?, ?)", text, sortOrder)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("focuses:")
	return res.LastInsertId()
}

func (m *FocusModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM focuses WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("focuses:")
	}
	return err
}

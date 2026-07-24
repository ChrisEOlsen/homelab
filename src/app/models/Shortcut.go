package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gova/app/cache"
)

type Shortcut struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Url       string `json:"url"`
	SortOrder int64  `json:"sort_order"`
	CreatedAt Time   `json:"created_at"`
}

type ShortcutPage struct {
	Items []Shortcut `json:"items"`
	Total int        `json:"total"`
}

var ShortcutAllowedColumns = []string{"id", "title", "url", "sort_order", "created_at"}

type ShortcutModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewShortcutModel(readDB, writeDB *sql.DB, c *cache.Cache) *ShortcutModel {
	return &ShortcutModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *ShortcutModel) GetPage(limit, offset int, opts QueryOpts) ([]Shortcut, int, error) {
	orderBy := "ORDER BY sort_order ASC, created_at ASC"
	if opts.Sort != "" {
		ob, err := orderByClause(opts.Sort, ShortcutAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		orderBy = ob
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, err := filterField(opts.FilterField, ShortcutAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("shortcuts:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page ShortcutPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM shortcuts"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, title, url, sort_order, created_at FROM shortcuts" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []Shortcut{}
	for rows.Next() {
		var item Shortcut
		if err := rows.Scan(&item.ID, &item.Title, &item.Url, &item.SortOrder, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(ShortcutPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *ShortcutModel) Find(id int64) (*Shortcut, error) {
	row := m.readDB.QueryRow("SELECT id, title, url, sort_order, created_at FROM shortcuts WHERE id = ?", id)
	var item Shortcut
	if err := row.Scan(&item.ID, &item.Title, &item.Url, &item.SortOrder, &item.CreatedAt); err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *ShortcutModel) Create(title, url string, sortOrder int64) (int64, error) {
	res, err := m.writeDB.Exec("INSERT INTO shortcuts (title, url, sort_order) VALUES (?, ?, ?)", title, url, sortOrder)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("shortcuts:")
	return res.LastInsertId()
}

func (m *ShortcutModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM shortcuts WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("shortcuts:")
	}
	return err
}

func (m *ShortcutModel) UpdateSortOrder(id int64, sortOrder int) error {
	_, err := m.writeDB.Exec("UPDATE shortcuts SET sort_order = ? WHERE id = ?", sortOrder, id)
	if err == nil {
		m.cache.Bust("shortcuts:")
	}
	return err
}

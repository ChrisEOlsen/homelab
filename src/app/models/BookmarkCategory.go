package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gova/app/cache"
)

type BookmarkCategory struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	CreatedAt Time   `json:"created_at"`
}

type BookmarkCategoryPage struct {
	Items []BookmarkCategory `json:"items"`
	Total int                `json:"total"`
}

var BookmarkCategoryAllowedColumns = []string{"id", "title", "created_at"}

type BookmarkCategoryModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewBookmarkCategoryModel(readDB, writeDB *sql.DB, c *cache.Cache) *BookmarkCategoryModel {
	return &BookmarkCategoryModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *BookmarkCategoryModel) GetPage(limit, offset int, opts QueryOpts) ([]BookmarkCategory, int, error) {
	orderBy := "ORDER BY created_at DESC"
	if opts.Sort != "" {
		ob, err := orderByClause(opts.Sort, BookmarkCategoryAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		orderBy = ob
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, err := filterField(opts.FilterField, BookmarkCategoryAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("bookmark_categories:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page BookmarkCategoryPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM bookmark_categories"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, title, created_at FROM bookmark_categories" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []BookmarkCategory{}
	for rows.Next() {
		var item BookmarkCategory
		if err := rows.Scan(&item.ID, &item.Title, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(BookmarkCategoryPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *BookmarkCategoryModel) Find(id int64) (*BookmarkCategory, error) {
	row := m.readDB.QueryRow("SELECT id, title, created_at FROM bookmark_categories WHERE id = ?", id)
	var item BookmarkCategory
	if err := row.Scan(&item.ID, &item.Title, &item.CreatedAt); err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *BookmarkCategoryModel) Create(title string) (int64, error) {
	res, err := m.writeDB.Exec("INSERT INTO bookmark_categories (title) VALUES (?)", title)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("bookmark_categories:")
	return res.LastInsertId()
}

func (m *BookmarkCategoryModel) Update(id int64, title string) error {
	res, err := m.writeDB.Exec("UPDATE bookmark_categories SET title = ? WHERE id = ?", title, id)
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
	m.cache.Bust("bookmark_categories:")
	return nil
}

func (m *BookmarkCategoryModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM bookmark_categories WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("bookmark_categories:")
		// Bookmarks cascade on category delete — bust their cache too.
		m.cache.Bust("bookmarks:")
	}
	return err
}

package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gova/app/cache"
)

type Bookmark struct {
	ID          int64   `json:"id"`
	CategoryId  int64   `json:"category_id"`
	Title       string  `json:"title"`
	Url         string  `json:"url"`
	Description *string `json:"description"`
	CreatedAt   Time    `json:"created_at"`
}

type BookmarkPage struct {
	Items []Bookmark `json:"items"`
	Total int        `json:"total"`
}

var BookmarkAllowedColumns = []string{"id", "category_id", "title", "url", "description", "created_at"}

type BookmarkModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewBookmarkModel(readDB, writeDB *sql.DB, c *cache.Cache) *BookmarkModel {
	return &BookmarkModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *BookmarkModel) GetPage(limit, offset int, opts QueryOpts) ([]Bookmark, int, error) {
	orderBy := "ORDER BY created_at DESC"
	if opts.Sort != "" {
		ob, err := orderByClause(opts.Sort, BookmarkAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		orderBy = ob
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, err := filterField(opts.FilterField, BookmarkAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("bookmarks:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page BookmarkPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM bookmarks"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, category_id, title, url, description, created_at FROM bookmarks" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []Bookmark{}
	for rows.Next() {
		var item Bookmark
		var descriptionNull sql.NullString
		if err := rows.Scan(&item.ID, &item.CategoryId, &item.Title, &item.Url, &descriptionNull, &item.CreatedAt); err != nil {
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

	if data, err := json.Marshal(BookmarkPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *BookmarkModel) Find(id int64) (*Bookmark, error) {
	row := m.readDB.QueryRow("SELECT id, category_id, title, url, description, created_at FROM bookmarks WHERE id = ?", id)
	var item Bookmark
	var descriptionNull sql.NullString
	if err := row.Scan(&item.ID, &item.CategoryId, &item.Title, &item.Url, &descriptionNull, &item.CreatedAt); err != nil {
		return nil, err
	}
	if descriptionNull.Valid {
		item.Description = &descriptionNull.String
	}
	return &item, nil
}

func (m *BookmarkModel) Create(categoryID int64, title, url string, description *string) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO bookmarks (category_id, title, url, description) VALUES (?, ?, ?, ?)",
		categoryID, title, url, description,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("bookmarks:")
	return res.LastInsertId()
}

func (m *BookmarkModel) Update(id, categoryID int64, title, url string, description *string) error {
	res, err := m.writeDB.Exec(
		"UPDATE bookmarks SET category_id = ?, title = ?, url = ?, description = ? WHERE id = ?",
		categoryID, title, url, description, id,
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
	m.cache.Bust("bookmarks:")
	return nil
}

func (m *BookmarkModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM bookmarks WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("bookmarks:")
	}
	return err
}

package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gova/app/cache"
)

type LogCategory struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	SchemaDef string `json:"schema_def"`
	CreatedAt Time   `json:"created_at"`
}

type LogCategoryPage struct {
	Items []LogCategory `json:"items"`
	Total int           `json:"total"`
}

var LogCategoryAllowedColumns = []string{"id", "title", "schema_def", "created_at"}

type LogCategoryModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewLogCategoryModel(readDB, writeDB *sql.DB, c *cache.Cache) *LogCategoryModel {
	return &LogCategoryModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *LogCategoryModel) GetPage(limit, offset int, opts QueryOpts) ([]LogCategory, int, error) {
	orderBy := "ORDER BY created_at DESC"
	if opts.Sort != "" {
		ob, err := orderByClause(opts.Sort, LogCategoryAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		orderBy = ob
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, err := filterField(opts.FilterField, LogCategoryAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("log_categories:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page LogCategoryPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM log_categories"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, title, schema_def, created_at FROM log_categories" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []LogCategory{}
	for rows.Next() {
		var item LogCategory
		if err := rows.Scan(&item.ID, &item.Title, &item.SchemaDef, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(LogCategoryPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *LogCategoryModel) Find(id int64) (*LogCategory, error) {
	row := m.readDB.QueryRow("SELECT id, title, schema_def, created_at FROM log_categories WHERE id = ?", id)
	var item LogCategory
	if err := row.Scan(&item.ID, &item.Title, &item.SchemaDef, &item.CreatedAt); err != nil {
		return nil, err
	}
	return &item, nil
}

func (m *LogCategoryModel) Create(title, schemaDef string) (int64, error) {
	res, err := m.writeDB.Exec("INSERT INTO log_categories (title, schema_def) VALUES (?, ?)", title, schemaDef)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("log_categories:")
	return res.LastInsertId()
}

func (m *LogCategoryModel) Update(id int64, title, schemaDef string) error {
	res, err := m.writeDB.Exec("UPDATE log_categories SET title = ?, schema_def = ? WHERE id = ?", title, schemaDef, id)
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
	m.cache.Bust("log_categories:")
	return nil
}

func (m *LogCategoryModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM log_categories WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("log_categories:")
		// Log entries cascade on category delete — bust their cache too.
		m.cache.Bust("log_entries:")
	}
	return err
}

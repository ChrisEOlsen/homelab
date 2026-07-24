package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gova/app/cache"
)

type CodexEntry struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Language    *string `json:"language"`
	Code        string  `json:"code"`
	Description *string `json:"description"`
	Folder      string  `json:"folder"`
	CreatedAt   Time    `json:"created_at"`
}

type CodexEntryPage struct {
	Items []CodexEntry `json:"items"`
	Total int          `json:"total"`
}

var CodexEntryAllowedColumns = []string{"id", "title", "language", "code", "description", "folder", "created_at"}

type CodexEntryModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewCodexEntryModel(readDB, writeDB *sql.DB, c *cache.Cache) *CodexEntryModel {
	return &CodexEntryModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *CodexEntryModel) GetPage(limit, offset int, opts QueryOpts) ([]CodexEntry, int, error) {
	orderBy := "ORDER BY folder ASC, title ASC"
	if opts.Sort != "" {
		ob, err := orderByClause(opts.Sort, CodexEntryAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		orderBy = ob
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, err := filterField(opts.FilterField, CodexEntryAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("codex_entries:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page CodexEntryPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM codex_entries"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, title, language, code, description, folder, created_at FROM codex_entries" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []CodexEntry{}
	for rows.Next() {
		var item CodexEntry
		var languageNull, descriptionNull sql.NullString
		if err := rows.Scan(&item.ID, &item.Title, &languageNull, &item.Code, &descriptionNull, &item.Folder, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		if languageNull.Valid {
			item.Language = &languageNull.String
		}
		if descriptionNull.Valid {
			item.Description = &descriptionNull.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(CodexEntryPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *CodexEntryModel) Find(id int64) (*CodexEntry, error) {
	row := m.readDB.QueryRow("SELECT id, title, language, code, description, folder, created_at FROM codex_entries WHERE id = ?", id)
	var item CodexEntry
	var languageNull, descriptionNull sql.NullString
	if err := row.Scan(&item.ID, &item.Title, &languageNull, &item.Code, &descriptionNull, &item.Folder, &item.CreatedAt); err != nil {
		return nil, err
	}
	if languageNull.Valid {
		item.Language = &languageNull.String
	}
	if descriptionNull.Valid {
		item.Description = &descriptionNull.String
	}
	return &item, nil
}

func (m *CodexEntryModel) Create(title string, language *string, code string, description *string, folder string) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO codex_entries (title, language, code, description, folder) VALUES (?, ?, ?, ?, ?)",
		title, language, code, description, folder,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("codex_entries:")
	return res.LastInsertId()
}

func (m *CodexEntryModel) Update(id int64, title string, language *string, code string, description *string, folder string) error {
	res, err := m.writeDB.Exec(
		"UPDATE codex_entries SET title = ?, language = ?, code = ?, description = ?, folder = ? WHERE id = ?",
		title, language, code, description, folder, id,
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
	m.cache.Bust("codex_entries:")
	return nil
}

func (m *CodexEntryModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM codex_entries WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("codex_entries:")
	}
	return err
}

// RenameFolder moves every entry filed directly under oldPath, or nested
// beneath it (oldPath + "/..."), to the equivalent path under newPath.
// Folders aren't a separate table — a folder "exists" only as the common
// prefix of its entries' folder paths — so a rename is just a bulk string
// swap across matching rows.
func (m *CodexEntryModel) RenameFolder(oldPath, newPath string) error {
	tx, err := m.writeDB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("UPDATE codex_entries SET folder = ? WHERE folder = ?", newPath, oldPath); err != nil {
		return err
	}
	// LIKE with escaped _ and % so folder names containing those characters
	// don't act as SQL wildcards; ESCAPE '\' pairs with the escaping below.
	prefixPattern := escapeLike(oldPath) + "/%"
	if _, err := tx.Exec(
		"UPDATE codex_entries SET folder = ? || substr(folder, ?) WHERE folder LIKE ? ESCAPE '\\'",
		newPath, len(oldPath)+1, prefixPattern,
	); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	m.cache.Bust("codex_entries:")
	return nil
}

// DeleteFolder removes every entry filed directly under path or nested
// beneath it. Callers are expected to confirm with the user first — this
// deletes real snippet data with no undo.
func (m *CodexEntryModel) DeleteFolder(path string) error {
	prefixPattern := escapeLike(path) + "/%"
	_, err := m.writeDB.Exec(
		"DELETE FROM codex_entries WHERE folder = ? OR folder LIKE ? ESCAPE '\\'",
		path, prefixPattern,
	)
	if err == nil {
		m.cache.Bust("codex_entries:")
	}
	return err
}

func escapeLike(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' || c == '%' || c == '_' {
			out = append(out, '\\')
		}
		out = append(out, c)
	}
	return string(out)
}

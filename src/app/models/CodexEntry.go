package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type CodexEntry struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Language    string    `json:"language"`
	Code        string    `json:"code"`
	Description string    `json:"description"`
	Folder      string    `json:"folder"`
	CreatedAt   time.Time `json:"created_at"`
}

type CodexEntryModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewCodexEntryModel(readDB, writeDB *sql.DB, c *cache.Cache) *CodexEntryModel {
	return &CodexEntryModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *CodexEntryModel) GetAll() ([]CodexEntry, error) {
	const cacheKey = "codex_entries:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []CodexEntry
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, title, language, code, description, folder, created_at FROM codex_entries ORDER BY folder ASC, title ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []CodexEntry
	for rows.Next() {
		var item CodexEntry
		var language, description sql.NullString
		if err := rows.Scan(&item.ID, &item.Title, &language, &item.Code, &description, &item.Folder, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Language = language.String
		item.Description = description.String
		items = append(items, item)
	}
	if data, err := json.Marshal(items); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, nil
}

func (m *CodexEntryModel) Find(id int64) (*CodexEntry, error) {
	row := m.readDB.QueryRow("SELECT id, title, language, code, description, folder, created_at FROM codex_entries WHERE id = ?", id)
	var item CodexEntry
	var language, description sql.NullString
	err := row.Scan(&item.ID, &item.Title, &language, &item.Code, &description, &item.Folder, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	item.Language = language.String
	item.Description = description.String
	return &item, nil
}

func (m *CodexEntryModel) Create(title, language, code, description, folder string) (int64, error) {
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

func (m *CodexEntryModel) Update(id int64, title, language, code, description, folder string) error {
	_, err := m.writeDB.Exec(
		"UPDATE codex_entries SET title = ?, language = ?, code = ?, description = ?, folder = ? WHERE id = ?",
		title, language, code, description, folder, id,
	)
	if err == nil {
		m.cache.Bust("codex_entries:")
	}
	return err
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

package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gova/app/cache"
)

type JournalEntry struct {
	ID        int64   `json:"id"`
	Title     *string `json:"title"`
	Content   string  `json:"content"`
	Mood      string  `json:"mood"`
	EntryDate string  `json:"entry_date"`
	CreatedAt Time    `json:"created_at"`
}

type JournalEntryPage struct {
	Items []JournalEntry `json:"items"`
	Total int            `json:"total"`
}

var JournalEntryAllowedColumns = []string{"id", "title", "content", "mood", "entry_date", "created_at"}

type JournalEntryModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewJournalEntryModel(readDB, writeDB *sql.DB, c *cache.Cache) *JournalEntryModel {
	return &JournalEntryModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *JournalEntryModel) GetPage(limit, offset int, opts QueryOpts) ([]JournalEntry, int, error) {
	orderBy := "ORDER BY entry_date DESC, created_at DESC"
	if opts.Sort != "" {
		ob, err := orderByClause(opts.Sort, JournalEntryAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		orderBy = ob
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, err := filterField(opts.FilterField, JournalEntryAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("journal_entries:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page JournalEntryPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM journal_entries"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, title, content, mood, entry_date, created_at FROM journal_entries" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []JournalEntry{}
	for rows.Next() {
		var item JournalEntry
		var titleNull sql.NullString
		if err := rows.Scan(&item.ID, &titleNull, &item.Content, &item.Mood, &item.EntryDate, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		if titleNull.Valid {
			item.Title = &titleNull.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(JournalEntryPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *JournalEntryModel) Find(id int64) (*JournalEntry, error) {
	row := m.readDB.QueryRow("SELECT id, title, content, mood, entry_date, created_at FROM journal_entries WHERE id = ?", id)
	var item JournalEntry
	var titleNull sql.NullString
	if err := row.Scan(&item.ID, &titleNull, &item.Content, &item.Mood, &item.EntryDate, &item.CreatedAt); err != nil {
		return nil, err
	}
	if titleNull.Valid {
		item.Title = &titleNull.String
	}
	return &item, nil
}

func (m *JournalEntryModel) Create(title *string, content, mood, entryDate string) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO journal_entries (title, content, mood, entry_date) VALUES (?, ?, ?, ?)",
		title, content, mood, entryDate,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("journal_entries:")
	return res.LastInsertId()
}

func (m *JournalEntryModel) Update(id int64, title *string, content, mood, entryDate string) error {
	res, err := m.writeDB.Exec(
		"UPDATE journal_entries SET title = ?, content = ?, mood = ?, entry_date = ? WHERE id = ?",
		title, content, mood, entryDate, id,
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
	m.cache.Bust("journal_entries:")
	return nil
}

func (m *JournalEntryModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM journal_entries WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("journal_entries:")
	}
	return err
}

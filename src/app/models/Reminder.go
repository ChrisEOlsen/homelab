package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"gova/app/cache"
)

type Reminder struct {
	ID             int64   `json:"id"`
	Title          string  `json:"title"`
	RemindAt       string  `json:"remind_at"`
	RecurrenceType string  `json:"recurrence_type"`
	RecurrenceDays *string `json:"recurrence_days"`
	IsActive       bool    `json:"is_active"`
	CreatedAt      Time    `json:"created_at"`
}

type ReminderPage struct {
	Items []Reminder `json:"items"`
	Total int        `json:"total"`
}

var ReminderAllowedColumns = []string{"id", "title", "remind_at", "recurrence_type", "recurrence_days", "is_active", "created_at"}

type ReminderModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewReminderModel(readDB, writeDB *sql.DB, c *cache.Cache) *ReminderModel {
	return &ReminderModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *ReminderModel) GetPage(limit, offset int, opts QueryOpts) ([]Reminder, int, error) {
	orderBy := "ORDER BY remind_at ASC"
	if opts.Sort != "" {
		ob, err := orderByClause(opts.Sort, ReminderAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		orderBy = ob
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, err := filterField(opts.FilterField, ReminderAllowedColumns)
		if err != nil {
			return nil, 0, err
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("reminders:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page ReminderPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM reminders"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, title, remind_at, recurrence_type, recurrence_days, is_active, created_at FROM reminders" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []Reminder{}
	for rows.Next() {
		var item Reminder
		var recurrenceDaysNull sql.NullString
		if err := rows.Scan(&item.ID, &item.Title, &item.RemindAt, &item.RecurrenceType, &recurrenceDaysNull, &item.IsActive, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		if recurrenceDaysNull.Valid {
			item.RecurrenceDays = &recurrenceDaysNull.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(ReminderPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *ReminderModel) Find(id int64) (*Reminder, error) {
	row := m.readDB.QueryRow("SELECT id, title, remind_at, recurrence_type, recurrence_days, is_active, created_at FROM reminders WHERE id = ?", id)
	var item Reminder
	var recurrenceDaysNull sql.NullString
	if err := row.Scan(&item.ID, &item.Title, &item.RemindAt, &item.RecurrenceType, &recurrenceDaysNull, &item.IsActive, &item.CreatedAt); err != nil {
		return nil, err
	}
	if recurrenceDaysNull.Valid {
		item.RecurrenceDays = &recurrenceDaysNull.String
	}
	return &item, nil
}

func (m *ReminderModel) Create(title, remindAt, recurrenceType string, recurrenceDays *string, isActive bool) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO reminders (title, remind_at, recurrence_type, recurrence_days, is_active) VALUES (?, ?, ?, ?, ?)",
		title, remindAt, recurrenceType, recurrenceDays, isActive,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("reminders:")
	return res.LastInsertId()
}

func (m *ReminderModel) Update(id int64, title, remindAt, recurrenceType string, recurrenceDays *string, isActive bool) error {
	res, err := m.writeDB.Exec(
		"UPDATE reminders SET title = ?, remind_at = ?, recurrence_type = ?, recurrence_days = ?, is_active = ? WHERE id = ?",
		title, remindAt, recurrenceType, recurrenceDays, isActive, id,
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
	m.cache.Bust("reminders:")
	return nil
}

func (m *ReminderModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM reminders WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("reminders:")
	}
	return err
}

func (m *ReminderModel) Toggle(id int64) error {
	res, err := m.writeDB.Exec("UPDATE reminders SET is_active = NOT is_active WHERE id = ?", id)
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
	m.cache.Bust("reminders:")
	return nil
}

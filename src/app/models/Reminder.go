package models

import (
	"database/sql"
	"encoding/json"
	"time"
	"gova/app/cache"
)

type Reminder struct {
	ID        int64     `json:"id"`
	Title string `json:"title"`
	RemindAt string `json:"remind_at"`
	RecurrenceType string `json:"recurrence_type"`
	RecurrenceDays string `json:"recurrence_days"`
	IsActive bool `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type ReminderModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewReminderModel(readDB, writeDB *sql.DB, c *cache.Cache) *ReminderModel {
	return &ReminderModel{readDB: readDB, writeDB: writeDB, cache: c}
}

func (m *ReminderModel) GetAll() ([]Reminder, error) {
	const cacheKey = "reminders:all"
	if hit, ok := m.cache.Get(cacheKey); ok {
		var items []Reminder
		if err := json.Unmarshal(hit, &items); err == nil {
			return items, nil
		}
	}
	rows, err := m.readDB.Query("SELECT id, title, remind_at, recurrence_type, recurrence_days, is_active, created_at FROM reminders ORDER BY remind_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Reminder
	for rows.Next() {
		var item Reminder
		var recurrenceDays sql.NullString
		if err := rows.Scan(&item.ID, &item.Title, &item.RemindAt, &item.RecurrenceType, &recurrenceDays, &item.IsActive, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.RecurrenceDays = recurrenceDays.String
		items = append(items, item)
	}
	if data, err := json.Marshal(items); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, nil
}

func (m *ReminderModel) Find(id int64) (*Reminder, error) {
	row := m.readDB.QueryRow("SELECT id, title, remind_at, recurrence_type, recurrence_days, is_active, created_at FROM reminders WHERE id = ?", id)
	var item Reminder
	var recurrenceDays sql.NullString
	err := row.Scan(&item.ID, &item.Title, &item.RemindAt, &item.RecurrenceType, &recurrenceDays, &item.IsActive, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	item.RecurrenceDays = recurrenceDays.String
	return &item, nil
}

func (m *ReminderModel) Create(title string, remind_at string, recurrence_type string, recurrence_days string, is_active bool) (int64, error) {
	res, err := m.writeDB.Exec(
		"INSERT INTO reminders (title, remind_at, recurrence_type, recurrence_days, is_active) VALUES (?, ?, ?, ?, ?)",
		title, remind_at, recurrence_type, recurrence_days, is_active,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("reminders:")
	return res.LastInsertId()
}

func (m *ReminderModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM reminders WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("reminders:")
	}
	return err
}

func (m *ReminderModel) GetUpcoming(limit int) ([]Reminder, error) {
	rows, err := m.readDB.Query(
		"SELECT id, title, remind_at, recurrence_type, recurrence_days, is_active, created_at FROM reminders WHERE is_active = 1 ORDER BY remind_at ASC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Reminder
	for rows.Next() {
		var item Reminder
		var recurrenceDays sql.NullString
		if err := rows.Scan(&item.ID, &item.Title, &item.RemindAt, &item.RecurrenceType, &recurrenceDays, &item.IsActive, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.RecurrenceDays = recurrenceDays.String
		items = append(items, item)
	}
	return items, nil
}

func (m *ReminderModel) Update(id int64, title, remindAt, recurrenceType, recurrenceDays string, isActive bool) error {
	// notified_at is reset on every edit: an edited reminder (new time, new
	// title, re-activated, etc.) should always be eligible for a fresh
	// notification going forward, matching user intent for "I changed this."
	_, err := m.writeDB.Exec(
		"UPDATE reminders SET title = ?, remind_at = ?, recurrence_type = ?, recurrence_days = ?, is_active = ?, notified_at = NULL WHERE id = ?",
		title, remindAt, recurrenceType, recurrenceDays, isActive, id,
	)
	if err == nil {
		m.cache.Bust("reminders:")
	}
	return err
}

// GetDueUnnotified returns active reminders whose remind_at has passed and
// that haven't been pushed yet. One-shot only — matches the fact that
// recurrence_type/recurrence_days are stored but never auto-advance
// remind_at anywhere else in this app either.
//
// remind_at is stored as a naive "YYYY-MM-DDTHH:MM" string with no timezone
// (straight from the browser's <input type="datetime-local">). Comparison
// is done in Go using time.Local (set via the TZ env var — see .env) rather
// than in SQL, since a raw string comparison against a Go-formatted "now"
// is fragile whenever seconds are present on one side and not the other.
func (m *ReminderModel) GetDueUnnotified() ([]Reminder, error) {
	rows, err := m.readDB.Query(
		"SELECT id, title, remind_at, recurrence_type, recurrence_days, is_active, created_at FROM reminders WHERE is_active = 1 AND notified_at IS NULL",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Reminder
	for rows.Next() {
		var item Reminder
		var recurrenceDays sql.NullString
		if err := rows.Scan(&item.ID, &item.Title, &item.RemindAt, &item.RecurrenceType, &recurrenceDays, &item.IsActive, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.RecurrenceDays = recurrenceDays.String
		due, err := time.ParseInLocation("2006-01-02T15:04", item.RemindAt, time.Local)
		if err != nil || due.After(time.Now()) {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (m *ReminderModel) MarkNotified(id int64) error {
	_, err := m.writeDB.Exec("UPDATE reminders SET notified_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("reminders:")
	}
	return err
}

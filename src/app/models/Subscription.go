package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
	"gova/app/cache"
)

type Subscription struct {
	ID        int64     `json:"id"`
	Name string `json:"name"`
	AmountCents int64 `json:"amount_cents"`
	Cadence string `json:"cadence"`
	BillingDay *int64 `json:"billing_day"`
	IsActive bool `json:"is_active"`
	StartedOn string `json:"started_on"`
	EndedOn *string `json:"ended_on"`
	Notes *string `json:"notes"`
	CreatedAt Time `json:"created_at"`
}

// subscriptionPage is the cache payload for a single page of results — items and
// total travel together so a cache hit does not need a second COUNT query.
type subscriptionPage struct {
	Items []Subscription `json:"items"`
	Total int               `json:"total"`
}

type SubscriptionModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewSubscriptionModel(readDB, writeDB *sql.DB, c *cache.Cache) *SubscriptionModel {
	return &SubscriptionModel{readDB: readDB, writeDB: writeDB, cache: c}
}

// subscriptionAllowedColumns is the whitelist for sort/filter — the model's real
// columns. orderByClause/filterField reject anything not in this list, so the
// only column names ever placed into SQL come from here.
var subscriptionAllowedColumns = []string{"id", "name", "amount_cents", "cadence", "billing_day", "is_active", "started_on", "ended_on", "notes", "created_at"}

// GetPage returns one window of rows plus the total (of the filtered set).
// Callers clamp limit and offset — see handlers/paging.go. Sort/filter columns
// are validated against subscriptionAllowedColumns; an unknown column yields
// ErrInvalidQuery (handlers map it to 422).
func (m *SubscriptionModel) GetPage(limit, offset int, opts QueryOpts) ([]Subscription, int, error) {
	orderBy, err := orderByClause(opts.Sort, subscriptionAllowedColumns)
	if err != nil {
		return nil, 0, err
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, ferr := filterField(opts.FilterField, subscriptionAllowedColumns)
		if ferr != nil {
			return nil, 0, ferr
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("subscriptions:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page subscriptionPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM subscriptions"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, name, amount_cents, cadence, billing_day, is_active, started_on, ended_on, notes, created_at FROM subscriptions" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// Initialized non-nil: a nil slice marshals as JSON null, which breaks
	// strictly-typed clients decoding an array.
	items := []Subscription{}
	for rows.Next() {
		var item Subscription
		var billing_dayNull sql.NullInt64
		var ended_onNull sql.NullString
		var notesNull sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &item.AmountCents, &item.Cadence, &billing_dayNull, &item.IsActive, &item.StartedOn, &ended_onNull, &notesNull, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		if billing_dayNull.Valid {
			item.BillingDay = &billing_dayNull.Int64
		}
		if ended_onNull.Valid {
			item.EndedOn = &ended_onNull.String
		}
		if notesNull.Valid {
			item.Notes = &notesNull.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(subscriptionPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *SubscriptionModel) Find(id int64) (*Subscription, error) {
	row := m.readDB.QueryRow("SELECT id, name, amount_cents, cadence, billing_day, is_active, started_on, ended_on, notes, created_at FROM subscriptions WHERE id = ?", id)
	var item Subscription
	var billing_dayNull sql.NullInt64
	var ended_onNull sql.NullString
	var notesNull sql.NullString
	if err := row.Scan(&item.ID, &item.Name, &item.AmountCents, &item.Cadence, &billing_dayNull, &item.IsActive, &item.StartedOn, &ended_onNull, &notesNull, &item.CreatedAt); err != nil {
		return nil, err
	}
	if billing_dayNull.Valid {
		item.BillingDay = &billing_dayNull.Int64
	}
	if ended_onNull.Valid {
		item.EndedOn = &ended_onNull.String
	}
	if notesNull.Valid {
		item.Notes = &notesNull.String
	}
	return &item, nil
}

func (m *SubscriptionModel) Create(name string, amount_cents int64, cadence string, billing_day *int64, is_active bool, started_on string, ended_on *string, notes *string) (int64, error) {
	// A nil pointer binds to SQL NULL directly — no sql.Null* wrapper needed
	// on the insert path.
	res, err := m.writeDB.Exec(
		"INSERT INTO subscriptions (name, amount_cents, cadence, billing_day, is_active, started_on, ended_on, notes) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		name, amount_cents, cadence, billing_day, is_active, started_on, ended_on, notes,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("subscriptions:")
	return res.LastInsertId()
}

func (m *SubscriptionModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM subscriptions WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("subscriptions:")
	}
	return err
}


func (m *SubscriptionModel) Update(id int64, name string, amount_cents int64, cadence string, billing_day *int64, is_active bool, started_on string, ended_on *string, notes *string) error {
	res, err := m.writeDB.Exec(
		"UPDATE subscriptions SET name = ?, amount_cents = ?, cadence = ?, billing_day = ?, is_active = ?, started_on = ?, ended_on = ?, notes = ? WHERE id = ?",
		name, amount_cents, cadence, billing_day, is_active, started_on, ended_on, notes, id,
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
	m.cache.Bust("subscriptions:")
	return nil
}


package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
	"gova/app/cache"
)

type Expense struct {
	ID        int64     `json:"id"`
	Name string `json:"name"`
	AmountCents int64 `json:"amount_cents"`
	Category *string `json:"category"`
	Status string `json:"status"`
	IncurredOn string `json:"incurred_on"`
	Notes *string `json:"notes"`
	CreatedAt Time `json:"created_at"`
}

// expensePage is the cache payload for a single page of results — items and
// total travel together so a cache hit does not need a second COUNT query.
type expensePage struct {
	Items []Expense `json:"items"`
	Total int               `json:"total"`
}

type ExpenseModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewExpenseModel(readDB, writeDB *sql.DB, c *cache.Cache) *ExpenseModel {
	return &ExpenseModel{readDB: readDB, writeDB: writeDB, cache: c}
}

// expenseAllowedColumns is the whitelist for sort/filter — the model's real
// columns. orderByClause/filterField reject anything not in this list, so the
// only column names ever placed into SQL come from here.
var expenseAllowedColumns = []string{"id", "name", "amount_cents", "category", "status", "incurred_on", "notes", "created_at"}

// GetPage returns one window of rows plus the total (of the filtered set).
// Callers clamp limit and offset — see handlers/paging.go. Sort/filter columns
// are validated against expenseAllowedColumns; an unknown column yields
// ErrInvalidQuery (handlers map it to 422).
func (m *ExpenseModel) GetPage(limit, offset int, opts QueryOpts) ([]Expense, int, error) {
	orderBy, err := orderByClause(opts.Sort, expenseAllowedColumns)
	if err != nil {
		return nil, 0, err
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, ferr := filterField(opts.FilterField, expenseAllowedColumns)
		if ferr != nil {
			return nil, 0, ferr
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("expenses:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page expensePage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM expenses"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, name, amount_cents, category, status, incurred_on, notes, created_at FROM expenses" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// Initialized non-nil: a nil slice marshals as JSON null, which breaks
	// strictly-typed clients decoding an array.
	items := []Expense{}
	for rows.Next() {
		var item Expense
		var categoryNull sql.NullString
		var notesNull sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &item.AmountCents, &categoryNull, &item.Status, &item.IncurredOn, &notesNull, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		if categoryNull.Valid {
			item.Category = &categoryNull.String
		}
		if notesNull.Valid {
			item.Notes = &notesNull.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(expensePage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *ExpenseModel) Find(id int64) (*Expense, error) {
	row := m.readDB.QueryRow("SELECT id, name, amount_cents, category, status, incurred_on, notes, created_at FROM expenses WHERE id = ?", id)
	var item Expense
	var categoryNull sql.NullString
	var notesNull sql.NullString
	if err := row.Scan(&item.ID, &item.Name, &item.AmountCents, &categoryNull, &item.Status, &item.IncurredOn, &notesNull, &item.CreatedAt); err != nil {
		return nil, err
	}
	if categoryNull.Valid {
		item.Category = &categoryNull.String
	}
	if notesNull.Valid {
		item.Notes = &notesNull.String
	}
	return &item, nil
}

func (m *ExpenseModel) Create(name string, amount_cents int64, category *string, status string, incurred_on string, notes *string) (int64, error) {
	// A nil pointer binds to SQL NULL directly — no sql.Null* wrapper needed
	// on the insert path.
	res, err := m.writeDB.Exec(
		"INSERT INTO expenses (name, amount_cents, category, status, incurred_on, notes) VALUES (?, ?, ?, ?, ?, ?)",
		name, amount_cents, category, status, incurred_on, notes,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("expenses:")
	return res.LastInsertId()
}

func (m *ExpenseModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM expenses WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("expenses:")
	}
	return err
}


func (m *ExpenseModel) Update(id int64, name string, amount_cents int64, category *string, status string, incurred_on string, notes *string) error {
	res, err := m.writeDB.Exec(
		"UPDATE expenses SET name = ?, amount_cents = ?, category = ?, status = ?, incurred_on = ?, notes = ? WHERE id = ?",
		name, amount_cents, category, status, incurred_on, notes, id,
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
	m.cache.Bust("expenses:")
	return nil
}


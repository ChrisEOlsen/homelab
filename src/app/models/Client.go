package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
	"gova/app/cache"
)

type Client struct {
	ID        int64     `json:"id"`
	Name string `json:"name"`
	MatchName string `json:"match_name"`
	Email *string `json:"email"`
	Phone *string `json:"phone"`
	RateCents int64 `json:"rate_cents"`
	Kind string `json:"kind"`
	IsActive bool `json:"is_active"`
	Notes *string `json:"notes"`
	CreatedAt Time `json:"created_at"`
}

// clientPage is the cache payload for a single page of results — items and
// total travel together so a cache hit does not need a second COUNT query.
type clientPage struct {
	Items []Client `json:"items"`
	Total int               `json:"total"`
}

type ClientModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewClientModel(readDB, writeDB *sql.DB, c *cache.Cache) *ClientModel {
	return &ClientModel{readDB: readDB, writeDB: writeDB, cache: c}
}

// clientAllowedColumns is the whitelist for sort/filter — the model's real
// columns. orderByClause/filterField reject anything not in this list, so the
// only column names ever placed into SQL come from here.
var clientAllowedColumns = []string{"id", "name", "match_name", "email", "phone", "rate_cents", "kind", "is_active", "notes", "created_at"}

// GetPage returns one window of rows plus the total (of the filtered set).
// Callers clamp limit and offset — see handlers/paging.go. Sort/filter columns
// are validated against clientAllowedColumns; an unknown column yields
// ErrInvalidQuery (handlers map it to 422).
func (m *ClientModel) GetPage(limit, offset int, opts QueryOpts) ([]Client, int, error) {
	orderBy, err := orderByClause(opts.Sort, clientAllowedColumns)
	if err != nil {
		return nil, 0, err
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, ferr := filterField(opts.FilterField, clientAllowedColumns)
		if ferr != nil {
			return nil, 0, ferr
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("clients:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page clientPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM clients"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, name, match_name, email, phone, rate_cents, kind, is_active, notes, created_at FROM clients" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// Initialized non-nil: a nil slice marshals as JSON null, which breaks
	// strictly-typed clients decoding an array.
	items := []Client{}
	for rows.Next() {
		var item Client
		var emailNull sql.NullString
		var phoneNull sql.NullString
		var notesNull sql.NullString
		if err := rows.Scan(&item.ID, &item.Name, &item.MatchName, &emailNull, &phoneNull, &item.RateCents, &item.Kind, &item.IsActive, &notesNull, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		if emailNull.Valid {
			item.Email = &emailNull.String
		}
		if phoneNull.Valid {
			item.Phone = &phoneNull.String
		}
		if notesNull.Valid {
			item.Notes = &notesNull.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(clientPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *ClientModel) Find(id int64) (*Client, error) {
	row := m.readDB.QueryRow("SELECT id, name, match_name, email, phone, rate_cents, kind, is_active, notes, created_at FROM clients WHERE id = ?", id)
	var item Client
	var emailNull sql.NullString
	var phoneNull sql.NullString
	var notesNull sql.NullString
	if err := row.Scan(&item.ID, &item.Name, &item.MatchName, &emailNull, &phoneNull, &item.RateCents, &item.Kind, &item.IsActive, &notesNull, &item.CreatedAt); err != nil {
		return nil, err
	}
	if emailNull.Valid {
		item.Email = &emailNull.String
	}
	if phoneNull.Valid {
		item.Phone = &phoneNull.String
	}
	if notesNull.Valid {
		item.Notes = &notesNull.String
	}
	return &item, nil
}

func (m *ClientModel) Create(name string, match_name string, email *string, phone *string, rate_cents int64, kind string, is_active bool, notes *string) (int64, error) {
	// A nil pointer binds to SQL NULL directly — no sql.Null* wrapper needed
	// on the insert path.
	res, err := m.writeDB.Exec(
		"INSERT INTO clients (name, match_name, email, phone, rate_cents, kind, is_active, notes) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		name, match_name, email, phone, rate_cents, kind, is_active, notes,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("clients:")
	return res.LastInsertId()
}

func (m *ClientModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM clients WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("clients:")
	}
	return err
}


func (m *ClientModel) Update(id int64, name string, match_name string, email *string, phone *string, rate_cents int64, kind string, is_active bool, notes *string) error {
	res, err := m.writeDB.Exec(
		"UPDATE clients SET name = ?, match_name = ?, email = ?, phone = ?, rate_cents = ?, kind = ?, is_active = ?, notes = ? WHERE id = ?",
		name, match_name, email, phone, rate_cents, kind, is_active, notes, id,
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
	m.cache.Bust("clients:")
	return nil
}

// ClientMatch is the projection the calendar sync needs: just enough to price a
// session. Kept separate from Client so the calendar package never imports a
// full model row, and so this query stays index-only cheap.
type ClientMatch struct {
	ID        int64
	MatchName string
	RateCents int
	Kind      string
}

// AllForMatching returns every active client for name resolution. The set is
// small (a handful of rows) and read once per sync, so it is not paginated.
func (m *ClientModel) AllForMatching() ([]ClientMatch, error) {
	rows, err := m.readDB.Query(
		"SELECT id, match_name, rate_cents, kind FROM clients WHERE is_active = 1",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []ClientMatch{}
	for rows.Next() {
		var c ClientMatch
		if err := rows.Scan(&c.ID, &c.MatchName, &c.RateCents, &c.Kind); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}


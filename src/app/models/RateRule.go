package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
	"gova/app/cache"
)

type RateRule struct {
	ID        int64     `json:"id"`
	DurationMin int64 `json:"duration_min"`
	AmountCents int64 `json:"amount_cents"`
	Label *string `json:"label"`
	CreatedAt Time `json:"created_at"`
}

// rate_rulePage is the cache payload for a single page of results — items and
// total travel together so a cache hit does not need a second COUNT query.
type rate_rulePage struct {
	Items []RateRule `json:"items"`
	Total int               `json:"total"`
}

type RateRuleModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewRateRuleModel(readDB, writeDB *sql.DB, c *cache.Cache) *RateRuleModel {
	return &RateRuleModel{readDB: readDB, writeDB: writeDB, cache: c}
}

// rate_ruleAllowedColumns is the whitelist for sort/filter — the model's real
// columns. orderByClause/filterField reject anything not in this list, so the
// only column names ever placed into SQL come from here.
var rate_ruleAllowedColumns = []string{"id", "duration_min", "amount_cents", "label", "created_at"}

// GetPage returns one window of rows plus the total (of the filtered set).
// Callers clamp limit and offset — see handlers/paging.go. Sort/filter columns
// are validated against rate_ruleAllowedColumns; an unknown column yields
// ErrInvalidQuery (handlers map it to 422).
func (m *RateRuleModel) GetPage(limit, offset int, opts QueryOpts) ([]RateRule, int, error) {
	orderBy, err := orderByClause(opts.Sort, rate_ruleAllowedColumns)
	if err != nil {
		return nil, 0, err
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, ferr := filterField(opts.FilterField, rate_ruleAllowedColumns)
		if ferr != nil {
			return nil, 0, ferr
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("rate_rules:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page rate_rulePage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM rate_rules"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, duration_min, amount_cents, label, created_at FROM rate_rules" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// Initialized non-nil: a nil slice marshals as JSON null, which breaks
	// strictly-typed clients decoding an array.
	items := []RateRule{}
	for rows.Next() {
		var item RateRule
		var labelNull sql.NullString
		if err := rows.Scan(&item.ID, &item.DurationMin, &item.AmountCents, &labelNull, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		if labelNull.Valid {
			item.Label = &labelNull.String
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(rate_rulePage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *RateRuleModel) Find(id int64) (*RateRule, error) {
	row := m.readDB.QueryRow("SELECT id, duration_min, amount_cents, label, created_at FROM rate_rules WHERE id = ?", id)
	var item RateRule
	var labelNull sql.NullString
	if err := row.Scan(&item.ID, &item.DurationMin, &item.AmountCents, &labelNull, &item.CreatedAt); err != nil {
		return nil, err
	}
	if labelNull.Valid {
		item.Label = &labelNull.String
	}
	return &item, nil
}

func (m *RateRuleModel) Create(duration_min int64, amount_cents int64, label *string) (int64, error) {
	// A nil pointer binds to SQL NULL directly — no sql.Null* wrapper needed
	// on the insert path.
	res, err := m.writeDB.Exec(
		"INSERT INTO rate_rules (duration_min, amount_cents, label) VALUES (?, ?, ?)",
		duration_min, amount_cents, label,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("rate_rules:")
	return res.LastInsertId()
}

func (m *RateRuleModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM rate_rules WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("rate_rules:")
	}
	return err
}


func (m *RateRuleModel) Update(id int64, duration_min int64, amount_cents int64, label *string) error {
	res, err := m.writeDB.Exec(
		"UPDATE rate_rules SET duration_min = ?, amount_cents = ?, label = ? WHERE id = ?",
		duration_min, amount_cents, label, id,
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
	m.cache.Bust("rate_rules:")
	return nil
}

// RateRuleRow is the projection the calendar sync prices against.
type RateRuleRow struct {
	DurationMin int
	AmountCents int
}

// AllRules returns every duration rule. Three rows in practice — read once per
// sync, not paginated.
func (m *RateRuleModel) AllRules() ([]RateRuleRow, error) {
	rows, err := m.readDB.Query("SELECT duration_min, amount_cents FROM rate_rules")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []RateRuleRow{}
	for rows.Next() {
		var r RateRuleRow
		if err := rows.Scan(&r.DurationMin, &r.AmountCents); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
	"gova/app/cache"
)

type TrainingSession struct {
	ID        int64     `json:"id"`
	Uid string `json:"uid"`
	Source string `json:"source"`
	ClientName string `json:"client_name"`
	ClientId *int64 `json:"client_id"`
	Service *string `json:"service"`
	SessionDate string `json:"session_date"`
	StartAt string `json:"start_at"`
	EndAt string `json:"end_at"`
	DurationMin int64 `json:"duration_min"`
	AmountCents int64 `json:"amount_cents"`
	RateSource string `json:"rate_source"`
	OverrideCents *int64 `json:"override_cents"`
	Status string `json:"status"`
	NeedsReview bool `json:"needs_review"`
	CreatedAt Time `json:"created_at"`
}

// training_sessionPage is the cache payload for a single page of results — items and
// total travel together so a cache hit does not need a second COUNT query.
type training_sessionPage struct {
	Items []TrainingSession `json:"items"`
	Total int               `json:"total"`
}

type TrainingSessionModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
	cache   *cache.Cache
}

func NewTrainingSessionModel(readDB, writeDB *sql.DB, c *cache.Cache) *TrainingSessionModel {
	return &TrainingSessionModel{readDB: readDB, writeDB: writeDB, cache: c}
}

// training_sessionAllowedColumns is the whitelist for sort/filter — the model's real
// columns. orderByClause/filterField reject anything not in this list, so the
// only column names ever placed into SQL come from here.
var training_sessionAllowedColumns = []string{"id", "uid", "source", "client_name", "client_id", "service", "session_date", "start_at", "end_at", "duration_min", "amount_cents", "rate_source", "override_cents", "status", "needs_review", "created_at"}

// GetPage returns one window of rows plus the total (of the filtered set).
// Callers clamp limit and offset — see handlers/paging.go. Sort/filter columns
// are validated against training_sessionAllowedColumns; an unknown column yields
// ErrInvalidQuery (handlers map it to 422).
func (m *TrainingSessionModel) GetPage(limit, offset int, opts QueryOpts) ([]TrainingSession, int, error) {
	orderBy, err := orderByClause(opts.Sort, training_sessionAllowedColumns)
	if err != nil {
		return nil, 0, err
	}
	where := ""
	args := []any{}
	if opts.FilterField != "" {
		col, ferr := filterField(opts.FilterField, training_sessionAllowedColumns)
		if ferr != nil {
			return nil, 0, ferr
		}
		where = " WHERE " + col + " = ?"
		args = append(args, opts.FilterValue)
	}

	cacheKey := fmt.Sprintf("training_sessions:page:%d:%d:%s:%s:%s", limit, offset, opts.Sort, opts.FilterField, opts.FilterValue)
	if hit, ok := m.cache.Get(cacheKey); ok {
		var page training_sessionPage
		if err := json.Unmarshal(hit, &page); err == nil {
			return page.Items, page.Total, nil
		}
	}

	var total int
	if err := m.readDB.QueryRow("SELECT COUNT(*) FROM training_sessions"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, uid, source, client_name, client_id, service, session_date, start_at, end_at, duration_min, amount_cents, rate_source, override_cents, status, needs_review, created_at FROM training_sessions" + where + " " + orderBy + " LIMIT ? OFFSET ?"
	rows, err := m.readDB.Query(query, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// Initialized non-nil: a nil slice marshals as JSON null, which breaks
	// strictly-typed clients decoding an array.
	items := []TrainingSession{}
	for rows.Next() {
		var item TrainingSession
		var client_idNull sql.NullInt64
		var serviceNull sql.NullString
		var override_centsNull sql.NullInt64
		if err := rows.Scan(&item.ID, &item.Uid, &item.Source, &item.ClientName, &client_idNull, &serviceNull, &item.SessionDate, &item.StartAt, &item.EndAt, &item.DurationMin, &item.AmountCents, &item.RateSource, &override_centsNull, &item.Status, &item.NeedsReview, &item.CreatedAt); err != nil {
			return nil, 0, err
		}
		if client_idNull.Valid {
			item.ClientId = &client_idNull.Int64
		}
		if serviceNull.Valid {
			item.Service = &serviceNull.String
		}
		if override_centsNull.Valid {
			item.OverrideCents = &override_centsNull.Int64
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if data, err := json.Marshal(training_sessionPage{Items: items, Total: total}); err == nil {
		m.cache.Set(cacheKey, data, 5*time.Minute)
	}
	return items, total, nil
}

func (m *TrainingSessionModel) Find(id int64) (*TrainingSession, error) {
	row := m.readDB.QueryRow("SELECT id, uid, source, client_name, client_id, service, session_date, start_at, end_at, duration_min, amount_cents, rate_source, override_cents, status, needs_review, created_at FROM training_sessions WHERE id = ?", id)
	var item TrainingSession
	var client_idNull sql.NullInt64
	var serviceNull sql.NullString
	var override_centsNull sql.NullInt64
	if err := row.Scan(&item.ID, &item.Uid, &item.Source, &item.ClientName, &client_idNull, &serviceNull, &item.SessionDate, &item.StartAt, &item.EndAt, &item.DurationMin, &item.AmountCents, &item.RateSource, &override_centsNull, &item.Status, &item.NeedsReview, &item.CreatedAt); err != nil {
		return nil, err
	}
	if client_idNull.Valid {
		item.ClientId = &client_idNull.Int64
	}
	if serviceNull.Valid {
		item.Service = &serviceNull.String
	}
	if override_centsNull.Valid {
		item.OverrideCents = &override_centsNull.Int64
	}
	return &item, nil
}

func (m *TrainingSessionModel) Create(uid string, source string, client_name string, client_id *int64, service *string, session_date string, start_at string, end_at string, duration_min int64, amount_cents int64, rate_source string, override_cents *int64, status string, needs_review bool) (int64, error) {
	// A nil pointer binds to SQL NULL directly — no sql.Null* wrapper needed
	// on the insert path.
	res, err := m.writeDB.Exec(
		"INSERT INTO training_sessions (uid, source, client_name, client_id, service, session_date, start_at, end_at, duration_min, amount_cents, rate_source, override_cents, status, needs_review) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		uid, source, client_name, client_id, service, session_date, start_at, end_at, duration_min, amount_cents, rate_source, override_cents, status, needs_review,
	)
	if err != nil {
		return 0, err
	}
	m.cache.Bust("training_sessions:")
	return res.LastInsertId()
}

func (m *TrainingSessionModel) Delete(id int64) error {
	_, err := m.writeDB.Exec("DELETE FROM training_sessions WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("training_sessions:")
	}
	return err
}


func (m *TrainingSessionModel) Update(id int64, uid string, source string, client_name string, client_id *int64, service *string, session_date string, start_at string, end_at string, duration_min int64, amount_cents int64, rate_source string, override_cents *int64, status string, needs_review bool) error {
	res, err := m.writeDB.Exec(
		"UPDATE training_sessions SET uid = ?, source = ?, client_name = ?, client_id = ?, service = ?, session_date = ?, start_at = ?, end_at = ?, duration_min = ?, amount_cents = ?, rate_source = ?, override_cents = ?, status = ?, needs_review = ? WHERE id = ?",
		uid, source, client_name, client_id, service, session_date, start_at, end_at, duration_min, amount_cents, rate_source, override_cents, status, needs_review, id,
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
	m.cache.Bust("training_sessions:")
	return nil
}


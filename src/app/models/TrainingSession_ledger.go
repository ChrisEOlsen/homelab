package models

import (
	"strings"
)

// TrainingSessionUpsert is one priced calendar event ready to be written.
// The calendar package builds it; this file is the only place that writes it.
type TrainingSessionUpsert struct {
	UID         string
	Source      string
	ClientName  string
	ClientID    *int64
	Service     *string
	SessionDate string
	StartAt     string
	EndAt       string
	DurationMin int
	AmountCents int
	RateSource  string
	Status      string
	NeedsReview bool
}

// UpsertFromCalendar writes one event, creating or updating by UID, and reports
// whether the row was new.
//
// Manual decisions outrank the feed on every re-sync:
//   - an override keeps its amount and its 'override' rate_source forever;
//   - a session marked 'ignored' by hand stays ignored;
//   - a re-seen session is un-cancelled, because the feed showing it again is
//     the authoritative signal that the appointment is back on.
func (m *TrainingSessionModel) UpsertFromCalendar(s TrainingSessionUpsert, now string) (bool, error) {
	var existing int
	if err := m.readDB.QueryRow(
		"SELECT COUNT(*) FROM training_sessions WHERE uid = ?", s.UID,
	).Scan(&existing); err != nil {
		return false, err
	}

	needsReview := 0
	if s.NeedsReview {
		needsReview = 1
	}

	_, err := m.writeDB.Exec(`
INSERT INTO training_sessions
    (uid, source, client_name, client_id, service, session_date, start_at, end_at,
     duration_min, amount_cents, rate_source, status, needs_review,
     first_seen_at, last_seen_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(uid) DO UPDATE SET
    source       = excluded.source,
    client_name  = excluded.client_name,
    client_id    = excluded.client_id,
    service      = excluded.service,
    session_date = excluded.session_date,
    start_at     = excluded.start_at,
    end_at       = excluded.end_at,
    duration_min = excluded.duration_min,
    amount_cents = CASE WHEN training_sessions.override_cents IS NOT NULL
                        THEN training_sessions.override_cents
                        ELSE excluded.amount_cents END,
    rate_source  = CASE WHEN training_sessions.override_cents IS NOT NULL
                        THEN 'override'
                        ELSE excluded.rate_source END,
    status       = CASE WHEN training_sessions.status = 'ignored'
                        THEN 'ignored'
                        ELSE excluded.status END,
    needs_review = CASE WHEN training_sessions.override_cents IS NOT NULL
                        THEN 0
                        ELSE excluded.needs_review END,
    last_seen_at = excluded.last_seen_at`,
		s.UID, s.Source, s.ClientName, s.ClientID, s.Service, s.SessionDate,
		s.StartAt, s.EndAt, s.DurationMin, s.AmountCents, s.RateSource, s.Status,
		needsReview, now, now,
	)
	if err != nil {
		return false, err
	}
	m.cache.Bust("training_sessions:")
	return existing == 0, nil
}

// CancelMissing marks every scheduled session inside the feed's own window that
// this run did not see as cancelled.
//
// The window bound is what makes the ledger permanent: the feed only covers the
// current week plus five, so anything dated before fromDate is frozen history
// the feed can no longer speak about. Manual rows are never cancelled — they are
// not in the feed by definition. Nothing is ever deleted.
func (m *TrainingSessionModel) CancelMissing(fromDate, toDate string, seenUIDs []string) (int, error) {
	q := `UPDATE training_sessions SET status = 'cancelled'
	      WHERE session_date >= ? AND session_date <= ?
	        AND source <> 'manual'
	        AND status = 'scheduled'`
	args := []any{fromDate, toDate}

	if len(seenUIDs) > 0 {
		q += " AND uid NOT IN (?" + strings.Repeat(", ?", len(seenUIDs)-1) + ")"
		for _, u := range seenUIDs {
			args = append(args, u)
		}
	}

	res, err := m.writeDB.Exec(q, args...)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	m.cache.Bust("training_sessions:")
	return int(n), nil
}

// ForMonth returns every non-cancelled session in a YYYY-MM month, earliest
// first — the rows the finances page paints.
func (m *TrainingSessionModel) ForMonth(month string) ([]TrainingSession, error) {
	rows, err := m.readDB.Query(`
SELECT id, uid, source, client_name, client_id, service, session_date, start_at,
       end_at, duration_min, amount_cents, rate_source, override_cents, status,
       needs_review, created_at
FROM training_sessions
WHERE session_date LIKE ? || '%' AND status <> 'cancelled'
ORDER BY start_at ASC`, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []TrainingSession{}
	for rows.Next() {
		var it TrainingSession
		if err := rows.Scan(
			&it.ID, &it.Uid, &it.Source, &it.ClientName, &it.ClientId, &it.Service,
			&it.SessionDate, &it.StartAt, &it.EndAt, &it.DurationMin, &it.AmountCents,
			&it.RateSource, &it.OverrideCents, &it.Status, &it.NeedsReview, &it.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// MonthIncomeResult separates money already worked for from money merely booked.
type MonthIncomeResult struct {
	EarnedCents    int
	ProjectedCents int
	SessionCount   int
	BySource       map[string]int
}

// MonthIncome totals a month. earned counts only sessions that have already
// finished (end_at <= now); projected counts the whole month including future
// bookings, which is why the page shows both — on the 3rd, projected is most of
// the month's calendar and earned is three days of work.
func (m *TrainingSessionModel) MonthIncome(month, now string) (MonthIncomeResult, error) {
	out := MonthIncomeResult{BySource: map[string]int{}}

	err := m.readDB.QueryRow(`
SELECT COALESCE(SUM(CASE WHEN end_at <= ? THEN amount_cents ELSE 0 END), 0),
       COALESCE(SUM(amount_cents), 0),
       COUNT(*)
FROM training_sessions
WHERE session_date LIKE ? || '%' AND status = 'scheduled'`,
		now, month,
	).Scan(&out.EarnedCents, &out.ProjectedCents, &out.SessionCount)
	if err != nil {
		return out, err
	}

	rows, err := m.readDB.Query(`
SELECT source, COALESCE(SUM(amount_cents), 0)
FROM training_sessions
WHERE session_date LIKE ? || '%' AND status = 'scheduled' AND end_at <= ?
GROUP BY source`, month, now)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	for rows.Next() {
		var src string
		var cents int
		if err := rows.Scan(&src, &cents); err != nil {
			return out, err
		}
		out.BySource[src] = cents
	}
	return out, rows.Err()
}

// AllTimeEarned totals every finished, non-cancelled session ever recorded.
func (m *TrainingSessionModel) AllTimeEarned(now string) (int, error) {
	var cents int
	err := m.readDB.QueryRow(
		"SELECT COALESCE(SUM(amount_cents), 0) FROM training_sessions WHERE status = 'scheduled' AND end_at <= ?",
		now,
	).Scan(&cents)
	return cents, err
}

// NeedsReviewCount is all-time, not per-month: a flagged session in a month you
// are not looking at is exactly the one you would otherwise miss.
func (m *TrainingSessionModel) NeedsReviewCount() (int, error) {
	var n int
	err := m.readDB.QueryRow(
		"SELECT COUNT(*) FROM training_sessions WHERE needs_review = 1 AND status = 'scheduled'",
	).Scan(&n)
	return n, err
}

// UnmatchedNames lists calendar names that priced without a client row behind
// them — the "add as client" queue.
func (m *TrainingSessionModel) UnmatchedNames(limit int) ([]string, error) {
	rows, err := m.readDB.Query(`
SELECT DISTINCT client_name FROM training_sessions
WHERE needs_review = 1 AND client_id IS NULL AND status = 'scheduled'
ORDER BY client_name LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

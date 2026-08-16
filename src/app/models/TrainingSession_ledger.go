package models

import (
	"strings"
	"time"
)

// monthRange turns "2026-08" into the half-open range ["2026-08-01",
// "2026-09-01"). A range comparison uses idx_training_sessions_date;
// LIKE ? || '%' does not — it forces a full scan, and this ledger only ever
// grows.
func monthRange(month string) (start, end string) {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		// Callers validate the format; on anything unparseable return an
		// empty range rather than silently matching every row.
		return month, month
	}
	return t.Format("2006-01-02"), t.AddDate(0, 1, 0).Format("2006-01-02")
}

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
//
// client_id and needs_review are feed-derived, not manual fields: they are
// recomputed from the clients table on every sync. To fix a wrong client_id,
// add or edit the *client* row so the name resolves — do not hand-edit the
// session. This is deliberate: guarding a hand-set client_id would strand a
// stale link when a client's match_name changes.
//
// Of the manual decisions, only status = 'ignored' survives a re-sync. A
// hand-set 'cancelled' is intentionally reverted, because the feed
// re-reporting an appointment is authoritative evidence it is back on. The
// UI must therefore write 'ignored', never 'cancelled', when a user parks a
// session.
//
// The sync is deliberately not transactional: it is ~130 autocommit writes
// plus a CancelMissing call, so a crash mid-sync leaves the ledger partly
// applied. That is safe here precisely because the upsert is idempotent and
// the next run re-applies the whole feed. Making it atomic would mean
// threading a *sql.Tx through every method.
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
//
// status = 'scheduled' is the live state and there is no completion
// transition — a past session stays 'scheduled' forever, and all four money
// aggregates in this file depend on that. Anyone adding a 'completed' status
// must update every aggregate here or silently zero the income.
func (m *TrainingSessionModel) CancelMissing(fromDate, toDate string, seenUIDs []string) (int, error) {
	if len(seenUIDs) == 0 {
		// An empty seen-set means the caller learned nothing, and learning
		// nothing is never evidence that everything was cancelled. Without
		// this guard the uid NOT IN (...) clause below would be omitted
		// entirely and this statement would cancel every scheduled session
		// in the window — e.g. a feed that 200s with an empty body would
		// silently wipe six weeks of booked income. The sync service has its
		// own zero-event guard; this is the second, independent belt at the
		// layer that actually holds the DELETE-shaped power.
		return 0, nil
	}

	q := `UPDATE training_sessions SET status = 'cancelled'
	      WHERE session_date >= ? AND session_date <= ?
	        AND source <> 'manual'
	        AND status = 'scheduled'
	        AND uid NOT IN (?` + strings.Repeat(", ?", len(seenUIDs)-1) + `)`
	args := []any{fromDate, toDate}
	for _, u := range seenUIDs {
		args = append(args, u)
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
//
// This includes 'ignored' rows (you must be able to see and un-ignore them),
// while the money aggregates below exclude them — so a caller that sums
// ForMonth's own rows will legitimately not match MonthIncome.
func (m *TrainingSessionModel) ForMonth(month string) ([]TrainingSession, error) {
	start, end := monthRange(month)
	rows, err := m.readDB.Query(`
SELECT id, uid, source, client_name, client_id, service, session_date, start_at,
       end_at, duration_min, amount_cents, rate_source, override_cents, status,
       needs_review, created_at
FROM training_sessions
WHERE session_date >= ? AND session_date < ? AND status <> 'cancelled'
ORDER BY start_at ASC`, start, end)
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
	EarnedBySource map[string]int
}

// MonthIncome totals a month. earned counts only sessions that have already
// finished (end_at <= now); projected counts the whole month including future
// bookings, which is why the page shows both — on the 3rd, projected is most of
// the month's calendar and earned is three days of work.
func (m *TrainingSessionModel) MonthIncome(month, now string) (MonthIncomeResult, error) {
	out := MonthIncomeResult{EarnedBySource: map[string]int{}}
	start, end := monthRange(month)

	err := m.readDB.QueryRow(`
SELECT COALESCE(SUM(CASE WHEN end_at <= ? THEN amount_cents ELSE 0 END), 0),
       COALESCE(SUM(amount_cents), 0),
       COUNT(*)
FROM training_sessions
WHERE session_date >= ? AND session_date < ? AND status = 'scheduled'`,
		now, start, end,
	).Scan(&out.EarnedCents, &out.ProjectedCents, &out.SessionCount)
	if err != nil {
		return out, err
	}

	rows, err := m.readDB.Query(`
SELECT source, COALESCE(SUM(amount_cents), 0)
FROM training_sessions
WHERE session_date >= ? AND session_date < ? AND status = 'scheduled' AND end_at <= ?
GROUP BY source`, start, end, now)
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
		out.EarnedBySource[src] = cents
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

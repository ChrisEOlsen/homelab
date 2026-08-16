package models

import "database/sql"

// MonthTotals splits a month's shopping into money already spent and money only
// planned. Planned items are reported separately and never reduce net — they
// drive the "committed" line on the page instead.
//
// Uses the half-open monthRange (see TrainingSession_ledger.go) rather than
// LIKE ? || '%' so the query can use idx_expenses_incurred_on instead of
// forcing a full table scan.
func (m *ExpenseModel) MonthTotals(month string) (bought, committed int, err error) {
	start, end := monthRange(month)
	err = m.readDB.QueryRow(`
SELECT COALESCE(SUM(CASE WHEN status =  'bought' THEN amount_cents ELSE 0 END), 0),
       COALESCE(SUM(CASE WHEN status <> 'bought' THEN amount_cents ELSE 0 END), 0)
FROM expenses WHERE incurred_on >= ? AND incurred_on < ?`, start, end).Scan(&bought, &committed)
	return bought, committed, err
}

// AllTimeBought totals every purchase ever recorded.
func (m *ExpenseModel) AllTimeBought() (int, error) {
	var cents int
	err := m.readDB.QueryRow(
		"SELECT COALESCE(SUM(amount_cents), 0) FROM expenses WHERE status = 'bought'",
	).Scan(&cents)
	return cents, err
}

// ForMonth returns a month's shopping rows, newest first.
//
// Uses the half-open monthRange rather than LIKE ? || '%' so the query can use
// idx_expenses_incurred_on instead of forcing a full table scan.
func (m *ExpenseModel) ForMonth(month string) ([]Expense, error) {
	start, end := monthRange(month)
	rows, err := m.readDB.Query(`
SELECT id, name, amount_cents, category, status, incurred_on, notes, created_at
FROM expenses WHERE incurred_on >= ? AND incurred_on < ?
ORDER BY incurred_on DESC, id DESC`, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []Expense{}
	for rows.Next() {
		var it Expense
		var categoryNull sql.NullString
		var notesNull sql.NullString
		if err := rows.Scan(&it.ID, &it.Name, &it.AmountCents, &categoryNull,
			&it.Status, &it.IncurredOn, &notesNull, &it.CreatedAt); err != nil {
			return nil, err
		}
		if categoryNull.Valid {
			it.Category = &categoryNull.String
		}
		if notesNull.Valid {
			it.Notes = &notesNull.String
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

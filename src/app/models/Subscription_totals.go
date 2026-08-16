package models

import (
	"math"
	"strconv"
)

// monthlyEquivalent normalizes any cadence to one month's cost.
func monthlyEquivalent(cadence string, amountCents int) int {
	switch cadence {
	case "yearly":
		return int(math.Round(float64(amountCents) / 12))
	case "weekly":
		return int(math.Round(float64(amountCents) * 52 / 12))
	default: // monthly
		return amountCents
	}
}

// monthsBetween counts inclusive YYYY-MM months from a to b; 0 if b precedes a.
func monthsBetween(a, b string) int {
	ay, am, ok1 := splitMonth(a)
	by, bm, ok2 := splitMonth(b)
	if !ok1 || !ok2 {
		return 0
	}
	n := (by-ay)*12 + (bm - am) + 1
	if n < 0 {
		return 0
	}
	return n
}

func splitMonth(s string) (year, month int, ok bool) {
	if len(s) < 7 {
		return 0, 0, false
	}
	y, err1 := strconv.Atoi(s[0:4])
	m, err2 := strconv.Atoi(s[5:7])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return y, m, true
}

type subRow struct {
	cadence   string
	amount    int
	startedOn string
	endedOn   *string
}

func (m *SubscriptionModel) allRows() ([]subRow, error) {
	rows, err := m.readDB.Query("SELECT cadence, amount_cents, started_on, ended_on FROM subscriptions")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []subRow{}
	for rows.Next() {
		var r subRow
		if err := rows.Scan(&r.cadence, &r.amount, &r.startedOn, &r.endedOn); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MonthlyEquivalentFor totals every subscription live during a YYYY-MM month.
// A subscription counts if it started on or before the month's last day and had
// not ended before the month's first day.
func (m *SubscriptionModel) MonthlyEquivalentFor(month string) (int, error) {
	rows, err := m.allRows()
	if err != nil {
		return 0, err
	}
	total := 0
	for _, r := range rows {
		if r.startedOn[:7] > month {
			continue
		}
		if r.endedOn != nil && len(*r.endedOn) >= 7 && (*r.endedOn)[:7] < month {
			continue
		}
		total += monthlyEquivalent(r.cadence, r.amount)
	}
	return total, nil
}

// TotalThrough sums every month each subscription was live, up to and including
// the given YYYY-MM. Counting each subscription's own active span is cheaper and
// exactly equivalent to looping every calendar month and re-summing.
func (m *SubscriptionModel) TotalThrough(month string) (int, error) {
	rows, err := m.allRows()
	if err != nil {
		return 0, err
	}
	total := 0
	for _, r := range rows {
		start := r.startedOn
		if len(start) >= 7 {
			start = start[:7]
		}
		end := month
		if r.endedOn != nil && len(*r.endedOn) >= 7 && (*r.endedOn)[:7] < month {
			end = (*r.endedOn)[:7]
		}
		total += monthlyEquivalent(r.cadence, r.amount) * monthsBetween(start, end)
	}
	return total, nil
}

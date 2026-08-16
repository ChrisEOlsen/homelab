package models

import (
	"fmt"
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

// monthPrefix returns the YYYY-MM prefix of a date string, and false if the
// value is too short to carry one. Month membership is decided by string
// comparison on these prefixes, so an unreadable date must drop out of the
// calculation rather than slice-panic or compare as an empty string that
// sorts before every real month.
func monthPrefix(date string) (string, bool) {
	if len(date) < 7 {
		return "", false
	}
	return date[:7], true
}

// deref flattens a nullable date column to a plain string, so callers can hand
// it straight to monthPrefix without repeating a nil check at every site.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// previousMonth steps a YYYY-MM back one, rolling the year at January.
func previousMonth(month string) string {
	y, m, ok := splitMonth(month)
	if !ok {
		return month
	}
	m--
	if m == 0 {
		m = 12
		y--
	}
	return fmt.Sprintf("%04d-%02d", y, m)
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
		startPrefix, ok := monthPrefix(r.startedOn)
		if !ok {
			continue
		}
		if startPrefix > month {
			continue
		}
		// Stopping a subscription drops it from the month you stop it in, not
		// just from the next one. The question this page answers is "if I stop
		// paying this, what do I have?", so a stop has to move the number you
		// are looking at. A subscription therefore counts for a month only if
		// it was still live at the end of it.
		if endedPrefix, ok := monthPrefix(deref(r.endedOn)); ok && endedPrefix <= month {
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
		start, ok := monthPrefix(r.startedOn)
		if !ok {
			continue
		}
		// Mirrors MonthlyEquivalentFor: the last month a stopped subscription
		// counts for is the one BEFORE the month it was stopped in, because a
		// stop takes effect immediately rather than at the month boundary.
		end := month
		if endedPrefix, ok := monthPrefix(deref(r.endedOn)); ok && endedPrefix <= month {
			end = previousMonth(endedPrefix)
		}
		total += monthlyEquivalent(r.cadence, r.amount) * monthsBetween(start, end)
	}
	return total, nil
}

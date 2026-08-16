package calendar

import (
	"strings"
	"unicode"
)

// IndependentDefaultCents is what a session on the own-booking-app source is
// worth when its client is not in the clients table yet. Everything on
// coachchrisfitness.com is an independent client, so pricing it immediately
// keeps the month correct even if the review queue is never opened.
const IndependentDefaultCents = 10000

// Client is the pricing projection of a clients row.
type Client struct {
	ID        int64
	MatchName string
	RateCents int
	Kind      string // "independent" or "ignored"
}

// RateRule is the duration fallback for gym sessions.
type RateRule struct {
	DurationMin int
	AmountCents int
}

// Pricing is what one session is worth and why.
type Pricing struct {
	ClientID    *int64
	AmountCents int
	// RateSource is "client" for two different cases that callers must not
	// conflate: a real match against the clients table, and the unmatched-cc
	// default (a new independent client priced at IndependentDefaultCents
	// and flagged for review). Only ClientID tells them apart — nil for the
	// default, set for a real match. Do not treat RateSource == "client"
	// alone as proof the session is tied to a known client row.
	RateSource  string // override | client | rule | unknown
	Status      string // scheduled | ignored
	NeedsReview bool
}

// NormalizeName is the one and only name-matching rule: trim, collapse internal
// whitespace, fold curly punctuation to ASCII, lowercase.
//
// Deliberately nothing wider — no substring, no surname, no initials. Ofer Rubin
// is an independent client at $100 and Ran Rubin is his father, a gym client
// priced by duration. Any looser rule silently prices Ran's sessions at $100 and
// inflates the month. TestOferAndRanRubinPriceDifferently guards this.
func NormalizeName(s string) string {
	var b strings.Builder
	pendingSpace := false

	for _, r := range s {
		switch r {
		case '’', '‘': // curly single quotes
			r = '\''
		case '–', '—': // en/em dash
			r = '-'
		}
		if unicode.IsSpace(r) {
			pendingSpace = true
			continue
		}
		if pendingSpace && b.Len() > 0 {
			b.WriteRune(' ')
		}
		pendingSpace = false
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// Resolve prices one session. The ladder, in order:
//
//  1. a manual override always wins;
//  2. an exact client-name match — an 'ignored' client zeroes and parks the row,
//     any other client uses its own rate;
//  3. no match on the own-app source: a new independent client, priced at the
//     default and flagged for review;
//  4. no match on the gym source: the duration rule table;
//  5. nothing fits (an odd-length block): zero, flagged for review.
func Resolve(source, clientName string, durationMin int, overrideCents *int, clients []Client, rules []RateRule) Pricing {
	if overrideCents != nil {
		return Pricing{AmountCents: *overrideCents, RateSource: "override", Status: "scheduled"}
	}

	key := NormalizeName(clientName)
	for _, c := range clients {
		if NormalizeName(c.MatchName) != key {
			continue
		}
		id := c.ID
		if c.Kind == "ignored" {
			return Pricing{ClientID: &id, AmountCents: 0, RateSource: "client", Status: "ignored"}
		}
		return Pricing{ClientID: &id, AmountCents: c.RateCents, RateSource: "client", Status: "scheduled"}
	}

	if source == "cc" {
		return Pricing{
			AmountCents: IndependentDefaultCents,
			RateSource:  "client",
			Status:      "scheduled",
			NeedsReview: true,
		}
	}

	for _, r := range rules {
		if r.DurationMin == durationMin {
			return Pricing{AmountCents: r.AmountCents, RateSource: "rule", Status: "scheduled"}
		}
	}

	return Pricing{AmountCents: 0, RateSource: "unknown", Status: "scheduled", NeedsReview: true}
}

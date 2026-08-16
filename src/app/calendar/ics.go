// Package calendar reads the calsync ICS feed and turns it into priced training
// sessions. Hand-written infrastructure: no MCP scaffold tool covers a protocol
// parser or a background sync service.
package calendar

import (
	"bufio"
	"io"
	"strings"
	"time"
	_ "time/tzdata" // embed the zone database; the runtime image may carry none
)

// FeedTimezone is the TZID every DTSTART/DTEND in the feed carries. calsync
// pins it in .env (TZID=America/New_York) and writes wall-clock stamps, so the
// ledger stores wall clock and never converts.
const FeedTimezone = "America/New_York"

// emDash is what calsync's lib/ics.js joins client and service with:
// [client, service].filter(Boolean).join(' — ').
const emDash = " — "

// Event is one VEVENT reduced to what the ledger needs.
type Event struct {
	UID        string
	Source     string // "wl" (gym) or "cc" (own booking app), from X-CALSYNC-SOURCE
	ClientName string
	Service    string
	Start      time.Time // wall clock in FeedTimezone; no zone attached
	End        time.Time
}

// DurationMin is the priced length of the appointment. WellnessLiving events
// carry a real end time; CC events are stamped at CC_DURATION_MIN by calsync
// because that UI has no end time at all.
func (e Event) DurationMin() int { return int(e.End.Sub(e.Start).Minutes()) }

// Date is the YYYY-MM-DD the session is grouped under.
func (e Event) Date() string { return e.Start.Format("2006-01-02") }

// StartAt and EndAt are the stored wall-clock stamps.
func (e Event) StartAt() string { return e.Start.Format("2006-01-02 15:04:05") }
func (e Event) EndAt() string   { return e.End.Format("2006-01-02 15:04:05") }

// Parse reads a whole ICS document. Malformed events are skipped rather than
// failing the run: one bad VEVENT must not cost a month of income.
func Parse(r io.Reader) ([]Event, error) {
	lines, err := unfold(r)
	if err != nil {
		return nil, err
	}

	out := []Event{}
	var cur *Event

	for _, line := range lines {
		switch line {
		case "BEGIN:VEVENT":
			cur = &Event{}
			continue
		case "END:VEVENT":
			if cur != nil && cur.UID != "" && !cur.Start.IsZero() {
				if cur.End.IsZero() || cur.End.Before(cur.Start) {
					cur.End = cur.Start
				}
				out = append(out, *cur)
			}
			cur = nil
			continue
		}
		if cur == nil {
			continue
		}

		name, value, ok := splitProperty(line)
		if !ok {
			continue
		}
		switch name {
		case "UID":
			cur.UID = value
		case "SUMMARY":
			cur.ClientName, cur.Service = splitSummary(unescape(value))
		case "DTSTART":
			if t, err := parseStamp(value); err == nil {
				cur.Start = t
			}
		case "DTEND":
			if t, err := parseStamp(value); err == nil {
				cur.End = t
			}
		case "X-CALSYNC-SOURCE":
			cur.Source = value
		}
	}
	return out, nil
}

// unfold reverses RFC 5545 line folding: a line beginning with a space or tab
// continues the previous one. calsync folds at 75 octets, so most descriptions
// and long client names arrive split across several physical lines. If a
// continuation line is the very first line in the document (nothing to fold
// into — already malformed input), its leading whitespace is stripped rather
// than left stuck onto the content, so a stray property name doesn't get a
// leading space baked in.
func unfold(r io.Reader) ([]string, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	lines := []string{}
	for sc.Scan() {
		// bufio.ScanLines already strips a trailing \r, so the CRLF the feed
		// uses per RFC 5545 needs no further trimming here.
		line := sc.Text()
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			if len(lines) > 0 {
				lines[len(lines)-1] += line[1:]
				continue
			}
			line = line[1:]
		}
		lines = append(lines, line)
	}
	return lines, sc.Err()
}

// splitProperty cuts NAME[;PARAMS]:VALUE at the first colon. The first colon is
// the right one even when the value contains more of them — a SUMMARY like
// "Women's Strength: Summer Skill Series" must not lose its second half.
func splitProperty(line string) (name, value string, ok bool) {
	left, value, ok := strings.Cut(line, ":")
	if !ok {
		return "", "", false
	}
	name, _, _ = strings.Cut(left, ";")
	return strings.ToUpper(name), value, true
}

// splitSummary separates client from service. A CC event has no service, so its
// whole summary is the client name.
func splitSummary(summary string) (client, service string) {
	if c, s, ok := strings.Cut(summary, emDash); ok {
		return strings.TrimSpace(c), strings.TrimSpace(s)
	}
	return strings.TrimSpace(summary), ""
}

// unescape reverses the escaping calsync's esc() applies to text values.
func unescape(v string) string {
	return strings.NewReplacer(`\n`, "\n", `\N`, "\n", `\,`, ",", `\;`, ";", `\\`, `\`).Replace(v)
}

// parseStamp reads a local date-time stamp. A trailing Z is tolerated even
// though calsync never writes one.
//
// Only the YYYYMMDDTHHMMSS form is accepted. An all-day event written as
// DTSTART;VALUE=DATE:20260810 (no "T", no time) fails to parse and is
// dropped silently by the BEGIN:VEVENT/END:VEVENT check in Parse (Start
// stays zero). This is intentional, not an oversight: the upstream calsync
// feed never emits all-day events, so there is nothing today to add support
// for.
func parseStamp(v string) (time.Time, error) {
	return time.Parse("20060102T150405", strings.TrimSuffix(v, "Z"))
}

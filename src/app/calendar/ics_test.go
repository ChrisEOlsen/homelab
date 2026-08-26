package calendar

import (
	"strings"
	"testing"
)

const fixture = "BEGIN:VCALENDAR\r\n" +
	"VERSION:2.0\r\n" +
	"BEGIN:VEVENT\r\n" +
	"UID:wl-124964949@calsync\r\n" +
	"SUMMARY:Patrick Sinclair — 45-Minute Training\r\n" +
	"DTSTART;TZID=America/New_York:20260810T070000\r\n" +
	"DTEND;TZID=America/New_York:20260810T074500\r\n" +
	"DESCRIPTION:45-Minute Training\\nwith Christopher Olsen\\nhttps://www.welln\r\n" +
	" essliving.com/rs/appointment-view.html?k_business=54598\r\n" +
	"X-CALSYNC-SOURCE:wl\r\n" +
	"END:VEVENT\r\n" +
	// A cc event in the CURRENT feed shape: calsync reads coachchrisfitness.com's
	// admin API, so the UID is that app's calendar_events.id, X-CALSYNC-ID
	// carries the same id, and DTEND is the real end (45 minutes here, not the
	// flat 60 the old DOM scraper had to assume).
	"BEGIN:VEVENT\r\n" +
	"UID:cc-35@calsync\r\n" +
	"SUMMARY:John Kublacki\r\n" +
	"DTSTART;TZID=America/New_York:20260810T110000\r\n" +
	"DTEND;TZID=America/New_York:20260810T114500\r\n" +
	"X-CALSYNC-SOURCE:cc\r\n" +
	"X-CALSYNC-ID:35\r\n" +
	"END:VEVENT\r\n" +
	"BEGIN:VEVENT\r\n" +
	"UID:wl-999@calsync\r\n" +
	"SUMMARY:(8/03-9/06) Women’s Strength: Summer Skill Series\r\n" +
	"DTSTART;TZID=America/New_York:20260812T180000\r\n" +
	"DTEND;TZID=America/New_York:20260812T190000\r\n" +
	"X-CALSYNC-SOURCE:wl\r\n" +
	"END:VEVENT\r\n" +
	"BEGIN:VEVENT\r\n" +
	"SUMMARY:No UID here\r\n" +
	"DTSTART;TZID=America/New_York:20260813T080000\r\n" +
	"END:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func TestParseFeed(t *testing.T) {
	events, err := Parse(strings.NewReader(fixture))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("want 3 usable events (the UID-less one skipped), got %d", len(events))
	}

	wl := events[0]
	if wl.ClientName != "Patrick Sinclair" || wl.Service != "45-Minute Training" {
		t.Fatalf("wl summary split wrong: client=%q service=%q", wl.ClientName, wl.Service)
	}
	if wl.Source != "wl" {
		t.Fatalf("source: want wl, got %q", wl.Source)
	}
	if wl.DurationMin() != 45 {
		t.Fatalf("duration: want 45, got %d", wl.DurationMin())
	}
	if wl.Date() != "2026-08-10" || wl.StartAt() != "2026-08-10 07:00:00" {
		t.Fatalf("stamps wrong: date=%q start=%q", wl.Date(), wl.StartAt())
	}

	cc := events[1]
	if cc.ClientName != "John Kublacki" || cc.Service != "" {
		t.Fatalf("cc summary should be all client: client=%q service=%q", cc.ClientName, cc.Service)
	}
	// 45, not 60: cc end times are real now. A regression here means either the
	// parser stopped reading DTEND or calsync went back to assuming a duration.
	if cc.DurationMin() != 45 {
		t.Fatalf("cc duration: want 45, got %d", cc.DurationMin())
	}
	if cc.SourceID != "35" {
		t.Fatalf("cc SourceID: want 35, got %q", cc.SourceID)
	}
	// wl carries no X-CALSYNC-ID in this fixture, and an absent id must read as
	// empty rather than inheriting the previous event's.
	if wl.SourceID != "" {
		t.Fatalf("wl SourceID: want empty, got %q", wl.SourceID)
	}

	// A summary containing its own colon must survive property splitting whole.
	class := events[2]
	if !strings.HasSuffix(class.ClientName, "Summer Skill Series") {
		t.Fatalf("colon in summary truncated the value: %q", class.ClientName)
	}
	if class.Service != "" {
		t.Fatalf("class has no em-dash, so no service: %q", class.Service)
	}
}

// TestMalformedEventMidFileLeavesNeighborsIntact proves a malformed VEVENT
// in the middle of the document (not just a trailing one, as in the main
// fixture) does not take out the valid events around it — Parse must keep
// scanning past END:VEVENT and pick the next BEGIN:VEVENT back up cleanly.
func TestMalformedEventMidFileLeavesNeighborsIntact(t *testing.T) {
	doc := "BEGIN:VCALENDAR\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:wl-before@calsync\r\n" +
		"SUMMARY:Before Person\r\n" +
		"DTSTART;TZID=America/New_York:20260810T070000\r\n" +
		"DTEND;TZID=America/New_York:20260810T074500\r\n" +
		"X-CALSYNC-SOURCE:wl\r\n" +
		"END:VEVENT\r\n" +
		"BEGIN:VEVENT\r\n" +
		"SUMMARY:No UID here\r\n" +
		"DTSTART;TZID=America/New_York:20260811T080000\r\n" +
		"END:VEVENT\r\n" +
		"BEGIN:VEVENT\r\n" +
		"UID:wl-after@calsync\r\n" +
		"SUMMARY:After Person\r\n" +
		"DTSTART;TZID=America/New_York:20260812T090000\r\n" +
		"DTEND;TZID=America/New_York:20260812T093000\r\n" +
		"X-CALSYNC-SOURCE:wl\r\n" +
		"END:VEVENT\r\n" +
		"END:VCALENDAR\r\n"

	events, err := Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("want 2 usable events (the mid-file UID-less one skipped), got %d: %+v", len(events), events)
	}
	if events[0].UID != "wl-before@calsync" {
		t.Errorf("first event: want wl-before@calsync, got %q", events[0].UID)
	}
	if events[1].UID != "wl-after@calsync" {
		t.Errorf("second event: want wl-after@calsync, got %q", events[1].UID)
	}
}

func TestUnfoldRejoinsContinuationLines(t *testing.T) {
	lines, err := unfold(strings.NewReader("DESCRIPTION:one\r\n two\r\nUID:x\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("want 2 logical lines, got %d: %q", len(lines), lines)
	}
	if lines[0] != "DESCRIPTION:onetwo" {
		t.Fatalf("unfold lost or added a character: %q", lines[0])
	}
}

func TestMissingEndFallsBackToStart(t *testing.T) {
	doc := "BEGIN:VEVENT\r\nUID:x\r\nSUMMARY:A Person\r\n" +
		"DTSTART;TZID=America/New_York:20260810T070000\r\nEND:VEVENT\r\n"
	events, err := Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].DurationMin() != 0 {
		t.Fatalf("want one zero-length event, got %+v", events)
	}
}

// TestEscapedTextRoundTrips verifies that calsync's text escaping is fully
// reversed rather than leaking a literal backslash onto the page — a client
// named "Smith, John" must never render as "Smith\, John". SUMMARY is checked
// through the public Parse/Event path since ClientName/Service are the only
// escaped text values Event exposes. DESCRIPTION's \n escaping (used in the
// real feed, see the wl fixture event above) has no field on Event to surface
// it through yet, so the newline case is checked directly against unescape,
// the single function responsible for reversing all of calsync's escaping.
func TestEscapedTextRoundTrips(t *testing.T) {
	doc := "BEGIN:VEVENT\r\n" +
		"UID:wl-esc@calsync\r\n" +
		"SUMMARY:Smith\\, John — Session\\; Extra\r\n" +
		"DTSTART;TZID=America/New_York:20260815T090000\r\n" +
		"DTEND;TZID=America/New_York:20260815T093000\r\n" +
		"X-CALSYNC-SOURCE:wl\r\n" +
		"END:VEVENT\r\n"

	events, err := Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	if events[0].ClientName != "Smith, John" {
		t.Fatalf("escaped comma survived unescaped: %q", events[0].ClientName)
	}
	if events[0].Service != "Session; Extra" {
		t.Fatalf("escaped semicolon survived unescaped: %q", events[0].Service)
	}

	const raw = `45-Minute Training\nwith Christopher Olsen`
	const want = "45-Minute Training\nwith Christopher Olsen"
	if got := unescape(raw); got != want {
		t.Fatalf("escaped newline survived unescaped: got %q want %q", got, want)
	}
}

// The ledger still holds rows written under calsync's previous cc UID scheme
// (a name-derived "cc-<stamp><Name>@calsync") for dates older than the
// reconcile window, and those rows are frozen history that can never be
// re-synced. The parser must therefore stay indifferent to the UID's shape and
// keep treating a feed without X-CALSYNC-ID as valid, so re-reading an archived
// feed never silently drops events.
func TestLegacyCCUIDStillParses(t *testing.T) {
	doc := "BEGIN:VEVENT\r\n" +
		"UID:cc-20260810110000JohnKublacki@calsync\r\n" +
		"SUMMARY:John Kublacki\r\n" +
		"DTSTART;TZID=America/New_York:20260810T110000\r\n" +
		"DTEND;TZID=America/New_York:20260810T120000\r\n" +
		"X-CALSYNC-SOURCE:cc\r\n" +
		"END:VEVENT\r\n"

	events, err := Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("want 1 event, got %d", len(events))
	}
	e := events[0]
	if e.UID != "cc-20260810110000JohnKublacki@calsync" {
		t.Errorf("legacy UID mangled: %q", e.UID)
	}
	if e.SourceID != "" {
		t.Errorf("legacy event has no X-CALSYNC-ID, want empty SourceID, got %q", e.SourceID)
	}
	if e.ClientName != "John Kublacki" || e.DurationMin() != 60 {
		t.Errorf("legacy event parsed wrong: client=%q duration=%d", e.ClientName, e.DurationMin())
	}
}

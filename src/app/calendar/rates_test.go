package calendar

import "testing"

func testClients() []Client {
	return []Client{
		{ID: 1, MatchName: "Ofer Rubin", RateCents: 10000, Kind: "independent"},
		{ID: 2, MatchName: "John Kublacki", RateCents: 10000, Kind: "independent"},
		{ID: 3, MatchName: "(8/03-9/06) Women’s Strength: Summer Skill Series", RateCents: 0, Kind: "ignored"},
	}
}

func testRules() []RateRule {
	return []RateRule{{30, 4500}, {45, 5000}, {60, 6000}}
}

// The load-bearing test of this whole feature. Ofer Rubin is an independent
// client at $100; Ran Rubin is his father and a gym client priced by duration.
// They share a surname, so any substring or surname match misprices Ran.
func TestOferAndRanRubinPriceDifferently(t *testing.T) {
	ofer := Resolve("cc", "Ofer Rubin", 60, nil, testClients(), testRules())
	if ofer.AmountCents != 10000 || ofer.RateSource != "client" {
		t.Fatalf("Ofer should price at his client rate: %+v", ofer)
	}
	if ofer.NeedsReview {
		t.Fatalf("a matched client needs no review: %+v", ofer)
	}

	ran := Resolve("wl", "Ran Rubin", 45, nil, testClients(), testRules())
	if ran.AmountCents != 5000 || ran.RateSource != "rule" {
		t.Fatalf("Ran must fall through to the 45-minute duration rule, got %+v", ran)
	}
	if ran.ClientID != nil {
		t.Fatalf("Ran must not be attached to a client row: %+v", ran)
	}
}

func TestOverrideBeatsEverything(t *testing.T) {
	override := 12345
	got := Resolve("cc", "Ofer Rubin", 60, &override, testClients(), testRules())
	if got.AmountCents != 12345 || got.RateSource != "override" {
		t.Fatalf("override ignored: %+v", got)
	}
}

func TestUnknownIndependentPricesAndFlags(t *testing.T) {
	got := Resolve("cc", "Brand New Person", 60, nil, testClients(), testRules())
	if got.AmountCents != IndependentDefaultCents {
		t.Fatalf("a cc session must price at the independent default, got %d", got.AmountCents)
	}
	if !got.NeedsReview {
		t.Fatalf("an unknown cc client must be flagged for review: %+v", got)
	}
	if got.ClientID != nil {
		t.Fatalf("no client row exists yet, so no id: %+v", got)
	}
}

func TestIgnoredClientParksTheRow(t *testing.T) {
	got := Resolve("wl", "(8/03-9/06) Women’s Strength: Summer Skill Series", 60, nil, testClients(), testRules())
	if got.Status != "ignored" || got.AmountCents != 0 {
		t.Fatalf("an ignored client must zero and park the session: %+v", got)
	}
}

func TestOddDurationFlagsForReview(t *testing.T) {
	got := Resolve("wl", "Someone New", 90, nil, testClients(), testRules())
	if got.AmountCents != 0 || got.RateSource != "unknown" || !got.NeedsReview {
		t.Fatalf("a 90-minute gym block has no rule and must be flagged: %+v", got)
	}
}

func TestNormalizeName(t *testing.T) {
	if NormalizeName("  Ofer   RUBIN ") != "ofer rubin" {
		t.Fatalf("trim/collapse/lowercase failed: %q", NormalizeName("  Ofer   RUBIN "))
	}
	if NormalizeName("Women’s") != NormalizeName("Women's") {
		t.Fatal("curly and straight apostrophes must fold together")
	}
	if NormalizeName("Ran Rubin") == NormalizeName("Ofer Rubin") {
		t.Fatal("normalization must never collapse two different people")
	}
}

// Regression test for the surname hazard: case and whitespace fold together,
// but a bare surname or first name is not an exact match and must not attach
// to Ofer Rubin's client row.
func TestNormalizeFoldsButNothingMore(t *testing.T) {
	folded := Resolve("cc", "  OFER   rubin  ", 60, nil, testClients(), testRules())
	if folded.AmountCents != 10000 || folded.RateSource != "client" || folded.ClientID == nil || *folded.ClientID != 1 {
		t.Fatalf("case/whitespace folding must still match Ofer Rubin exactly: %+v", folded)
	}

	surnameOnly := Resolve("wl", "Rubin", 60, nil, testClients(), testRules())
	if surnameOnly.ClientID != nil || surnameOnly.RateSource != "rule" || surnameOnly.AmountCents != 6000 {
		t.Fatalf("surname alone must not match Ofer Rubin's client row: %+v", surnameOnly)
	}

	firstNameOnly := Resolve("wl", "Ofer", 60, nil, testClients(), testRules())
	if firstNameOnly.ClientID != nil || firstNameOnly.RateSource != "rule" || firstNameOnly.AmountCents != 6000 {
		t.Fatalf("first name alone must not match Ofer Rubin's client row: %+v", firstNameOnly)
	}
}

// An ignored client must win over the duration-rule table even when its
// duration happens to coincide with a real rate rule.
func TestIgnoredClientBeatsDurationRule(t *testing.T) {
	got := Resolve("wl", "(8/03-9/06) Women’s Strength: Summer Skill Series", 45, nil, testClients(), testRules())
	if got.Status != "ignored" || got.AmountCents != 0 || got.RateSource != "client" {
		t.Fatalf("an ignored client must win over a same-duration rate rule: %+v", got)
	}
}

// An explicit override must win even over a client match that would
// otherwise be zeroed and parked as ignored.
func TestOverrideBeatsIgnoredClient(t *testing.T) {
	override := 500
	got := Resolve("wl", "(8/03-9/06) Women’s Strength: Summer Skill Series", 60, &override, testClients(), testRules())
	if got.AmountCents != 500 || got.RateSource != "override" {
		t.Fatalf("an override must win even over an ignored client: %+v", got)
	}
	if got.Status != "scheduled" {
		t.Fatalf("an override yields a scheduled session, not ignored: %+v", got)
	}
}

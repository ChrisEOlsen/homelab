package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gova/app/cache"
	"gova/app/calendar"
	"gova/app/db"
)

const financesSchema = calendarSyncSchema + `
CREATE TABLE expenses (
    id INTEGER PRIMARY KEY, name TEXT NOT NULL, amount_cents INTEGER NOT NULL,
    category TEXT, status TEXT NOT NULL DEFAULT 'planned', incurred_on TEXT NOT NULL,
    notes TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE subscriptions (
    id INTEGER PRIMARY KEY, name TEXT NOT NULL, amount_cents INTEGER NOT NULL,
    cadence TEXT NOT NULL DEFAULT 'monthly', billing_day INTEGER,
    is_active INTEGER NOT NULL DEFAULT 1, started_on TEXT NOT NULL, ended_on TEXT,
    notes TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP);
`

type summaryEnvelope struct {
	OK   bool `json:"ok"`
	Data struct {
		Month  string `json:"month"`
		Income struct {
			EarnedCents    int `json:"earned_cents"`
			ProjectedCents int `json:"projected_cents"`
		} `json:"income"`
		Spending struct {
			SubscriptionsCents     int `json:"subscriptions_cents"`
			ShoppingBoughtCents    int `json:"shopping_bought_cents"`
			ShoppingCommittedCents int `json:"shopping_committed_cents"`
		} `json:"spending"`
		NetCents                        int `json:"net_cents"`
		NetAfterCommittedCents          int `json:"net_after_committed_cents"`
		ProjectedNetCents               int `json:"projected_net_cents"`
		ProjectedNetAfterCommittedCents int `json:"projected_net_after_committed_cents"`
		AllTime                         struct {
			IncomeCents int `json:"income_cents"`
			SpendCents  int `json:"spend_cents"`
			NetCents    int `json:"net_cents"`
		} `json:"all_time"`
		Sessions []struct {
			UID    string `json:"uid"`
			Status string `json:"status"`
		} `json:"sessions"`
	} `json:"data"`
}

func TestFinancesSummaryMaths(t *testing.T) {
	d := db.OpenTest(t, financesSchema)

	// Two finished sessions ($100 + $50), one still in the future ($60).
	if _, err := d.Write.Exec(`
INSERT INTO training_sessions (uid, source, client_name, session_date, start_at, end_at, duration_min, amount_cents, rate_source, status) VALUES
 ('a','cc','Ofer Rubin','2026-08-03','2026-08-03 11:00:00','2026-08-03 12:00:00',60,10000,'client','scheduled'),
 ('b','wl','Someone',   '2026-08-04','2026-08-04 09:00:00','2026-08-04 09:45:00',45, 5000,'rule',  'scheduled'),
 ('c','wl','Later',     '2026-08-28','2026-08-28 09:00:00','2026-08-28 10:00:00',60, 6000,'rule',  'scheduled'),
 ('d','wl','Gone',      '2026-08-05','2026-08-05 09:00:00','2026-08-05 10:00:00',60, 6000,'rule',  'cancelled')`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Write.Exec(`
INSERT INTO expenses (name, amount_cents, status, incurred_on) VALUES
 ('Rack', 20000, 'bought',  '2026-08-02'),
 ('Bars',  3000, 'planned', '2026-08-06')`); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Write.Exec(`
INSERT INTO subscriptions (name, amount_cents, cadence, is_active, started_on) VALUES
 ('Spotify', 1200, 'monthly', 1, '2026-01-01')`); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/finances/summary?month=2026-08", nil)
	rec := httptest.NewRecorder()
	FinancesGET(d.Read, d.Write, cache.New())(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var env summaryEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Earned counts only finished sessions and excludes the cancelled one.
	if env.Data.Income.EarnedCents != 15000 {
		t.Fatalf("earned: want 15000, got %d", env.Data.Income.EarnedCents)
	}
	if env.Data.Income.ProjectedCents != 21000 {
		t.Fatalf("projected: want 21000, got %d", env.Data.Income.ProjectedCents)
	}
	if env.Data.Spending.ShoppingCommittedCents != 3000 {
		t.Fatalf("committed: want 3000, got %d", env.Data.Spending.ShoppingCommittedCents)
	}

	// net = earned - subscriptions - bought; planned never subtracts.
	if want := 15000 - 1200 - 20000; env.Data.NetCents != want {
		t.Fatalf("net: want %d, got %d", want, env.Data.NetCents)
	}
	if want := 15000 - 1200 - 20000 - 3000; env.Data.NetAfterCommittedCents != want {
		t.Fatalf("net after committed: want %d, got %d", want, env.Data.NetAfterCommittedCents)
	}

	// The forward-looking pair: the same arithmetic against projected income
	// rather than earned, which is what the Net tile's disclosure shows.
	if want := 21000 - 1200 - 20000; env.Data.ProjectedNetCents != want {
		t.Fatalf("projected net: want %d, got %d", want, env.Data.ProjectedNetCents)
	}
	if want := 21000 - 1200 - 20000 - 3000; env.Data.ProjectedNetAfterCommittedCents != want {
		t.Fatalf("projected net after committed: want %d, got %d",
			want, env.Data.ProjectedNetAfterCommittedCents)
	}

	// Projected income is never below earned, so the projected variants can
	// never be worse than their earned counterparts. If that inverts, the
	// earned/projected filters have drifted apart.
	if env.Data.ProjectedNetCents < env.Data.NetCents {
		t.Fatalf("projected net (%d) must not be below earned net (%d)",
			env.Data.ProjectedNetCents, env.Data.NetCents)
	}
}

func TestFinancesSummaryRejectsBadMonth(t *testing.T) {
	d := db.OpenTest(t, financesSchema)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/finances/summary?month=August", nil)
	rec := httptest.NewRecorder()
	FinancesGET(d.Read, d.Write, cache.New())(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", rec.Code)
	}
}

// TestFinancesSummaryExcludesCancelledIncludesIgnored guards the deliberate
// split between the money queries and the list query: MonthIncome (and every
// all-time aggregate) filters status = 'scheduled', so both 'cancelled' and
// 'ignored' sessions must be invisible to earned/projected. ForMonth filters
// only status <> 'cancelled', so a hand-ignored session must still show up in
// the sessions array the page renders — that is how a user finds and
// un-ignores it.
func TestFinancesSummaryExcludesCancelledIncludesIgnored(t *testing.T) {
	d := db.OpenTest(t, financesSchema)

	if _, err := d.Write.Exec(`
INSERT INTO training_sessions (uid, source, client_name, session_date, start_at, end_at, duration_min, amount_cents, rate_source, status) VALUES
 ('counted',   'cc','Real Client','2026-08-03','2026-08-03 11:00:00','2026-08-03 12:00:00',60,10000,'client','scheduled'),
 ('cancelled', 'wl','Cancelled',  '2026-08-04','2026-08-04 09:00:00','2026-08-04 10:00:00',60,99999,'rule',  'cancelled'),
 ('ignored',   'wl','Ignored',    '2026-08-05','2026-08-05 09:00:00','2026-08-05 10:00:00',60,88888,'rule',  'ignored')`); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/finances/summary?month=2026-08", nil)
	rec := httptest.NewRecorder()
	FinancesGET(d.Read, d.Write, cache.New())(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var env summaryEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Money figures see only the one 'scheduled' session, whether finished
	// (earned) or across the whole month (projected) — cancelled and ignored
	// contribute to neither.
	if env.Data.Income.EarnedCents != 10000 {
		t.Fatalf("earned: want 10000 (only the scheduled session), got %d", env.Data.Income.EarnedCents)
	}
	if env.Data.Income.ProjectedCents != 10000 {
		t.Fatalf("projected: want 10000 (only the scheduled session), got %d", env.Data.Income.ProjectedCents)
	}

	// The sessions list is a different filter: it excludes only 'cancelled',
	// so 'ignored' must still be present for the user to find and un-ignore.
	seen := map[string]bool{}
	for _, s := range env.Data.Sessions {
		seen[s.UID] = true
	}
	if !seen["counted"] {
		t.Fatal("sessions: expected the scheduled session to be present")
	}
	if !seen["ignored"] {
		t.Fatal("sessions: expected the ignored session to still appear in the list")
	}
	if seen["cancelled"] {
		t.Fatal("sessions: cancelled session must not appear in the list")
	}
}

// TestFinancesSummaryRejectsImpossibleMonth guards the calendar-parse fix:
// monthPattern's old shape-only regex accepted a right-shaped but impossible
// month like "2026-13" and let every downstream query silently match zero
// rows, returning a fully zeroed 200 instead of an error.
func TestFinancesSummaryRejectsImpossibleMonth(t *testing.T) {
	for _, month := range []string{"2026-13", "2026-00", "2026-99"} {
		t.Run(month, func(t *testing.T) {
			d := db.OpenTest(t, financesSchema)
			req := httptest.NewRequest(http.MethodGet, "/api/v1/finances/summary?month="+month, nil)
			rec := httptest.NewRecorder()
			FinancesGET(d.Read, d.Write, cache.New())(rec, req)

			if rec.Code != http.StatusUnprocessableEntity {
				t.Fatalf("want 422, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestFinancesSummaryAcceptsValidMonth checks the fix didn't overcorrect: a
// real YYYY-MM still works, and an omitted month still defaults to the
// current month.
func TestFinancesSummaryAcceptsValidMonth(t *testing.T) {
	d := db.OpenTest(t, financesSchema)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/finances/summary?month=2026-08", nil)
	rec := httptest.NewRecorder()
	FinancesGET(d.Read, d.Write, cache.New())(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var env summaryEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.Month != "2026-08" {
		t.Fatalf("month: want 2026-08, got %s", env.Data.Month)
	}

	d2 := db.OpenTest(t, financesSchema)
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/finances/summary", nil)
	rec2 := httptest.NewRecorder()
	FinancesGET(d2.Read, d2.Write, cache.New())(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec2.Code, rec2.Body.String())
	}
	var env2 summaryEnvelope
	if err := json.Unmarshal(rec2.Body.Bytes(), &env2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	wantMonth := calendar.Now().Format("2006-01")
	if env2.Data.Month != wantMonth {
		t.Fatalf("month: want current month %s, got %s", wantMonth, env2.Data.Month)
	}
}

// TestFinancesSummaryAllTimeSpendIsBrowseIndependent is the regression test
// for the all_time.spend_cents fix: TotalThrough must always be anchored to
// the current month, not the browsed month. Before the fix, browsing to an
// old month passed that old month into TotalThrough and understated all-time
// subscription spend by every month between then and now.
func TestFinancesSummaryAllTimeSpendIsBrowseIndependent(t *testing.T) {
	d := db.OpenTest(t, financesSchema)

	now := calendar.Now()
	oldMonth := now.AddDate(-3, 0, 0).Format("2006-01")
	startedOn := now.AddDate(-5, 0, 0).Format("2006-01-02")

	if _, err := d.Write.Exec(`
INSERT INTO subscriptions (name, amount_cents, cadence, is_active, started_on) VALUES
 ('Spotify', 1200, 'monthly', 1, ?)`, startedOn); err != nil {
		t.Fatal(err)
	}

	reqOld := httptest.NewRequest(http.MethodGet, "/api/v1/finances/summary?month="+oldMonth, nil)
	recOld := httptest.NewRecorder()
	FinancesGET(d.Read, d.Write, cache.New())(recOld, reqOld)
	if recOld.Code != http.StatusOK {
		t.Fatalf("old month: want 200, got %d: %s", recOld.Code, recOld.Body.String())
	}
	var envOld summaryEnvelope
	if err := json.Unmarshal(recOld.Body.Bytes(), &envOld); err != nil {
		t.Fatalf("decode old: %v", err)
	}

	reqNow := httptest.NewRequest(http.MethodGet, "/api/v1/finances/summary", nil)
	recNow := httptest.NewRecorder()
	FinancesGET(d.Read, d.Write, cache.New())(recNow, reqNow)
	if recNow.Code != http.StatusOK {
		t.Fatalf("current month: want 200, got %d: %s", recNow.Code, recNow.Body.String())
	}
	var envNow summaryEnvelope
	if err := json.Unmarshal(recNow.Body.Bytes(), &envNow); err != nil {
		t.Fatalf("decode now: %v", err)
	}

	if envOld.Data.AllTime.SpendCents != envNow.Data.AllTime.SpendCents {
		t.Fatalf("all_time.spend_cents must not depend on the browsed month: old-month=%d current-month=%d",
			envOld.Data.AllTime.SpendCents, envNow.Data.AllTime.SpendCents)
	}
	if envOld.Data.AllTime.SpendCents == 0 {
		t.Fatal("all_time.spend_cents should be nonzero given the seeded subscription")
	}
}

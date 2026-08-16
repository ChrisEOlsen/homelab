package handlers

import (
	"database/sql"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"gova/app/cache"
	"gova/app/calendar"
	"gova/app/models"
)

// financeTaxRateBP reads FINANCE_TAX_RATE_BP (basis points; 1800 = 18%). No
// MCP-managed settings table exists for this yet (the gova-builder server was
// unavailable when tax support was added), so the rate is env-configured
// until a proper settings UI can be scaffolded.
func financeTaxRateBP() int {
	if v := os.Getenv("FINANCE_TAX_RATE_BP"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return 1800
}

// financeTaxSources reads FINANCE_TAX_SOURCES (comma-separated training_session
// sources the tax rate applies to). Independent-client sessions (source 'cc')
// are excluded by default -- he registers those himself at year end -- but the
// set is configurable, not hardcoded, because that exclusion is described as
// "for now."
func financeTaxSources() map[string]bool {
	v := os.Getenv("FINANCE_TAX_SOURCES")
	if v == "" {
		v = "wl"
	}
	out := map[string]bool{}
	for _, s := range strings.Split(v, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out[s] = true
		}
	}
	return out
}

// taxSourceList flattens the taxed-source set for the wire, sorted so the
// payload is stable between requests (Go map order is not).
func taxSourceList(taxed map[string]bool) []string {
	out := make([]string, 0, len(taxed))
	for s := range taxed {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// financeTaxCents sums bySource cents for only the taxed sources, then applies
// rateBP with integer round-half-up arithmetic -- (cents*bp + 5000) / 10000 --
// never float, so tax is never off by a cent from a truncation.
func financeTaxCents(bySource map[string]int, taxed map[string]bool, rateBP int) int {
	var taxableCents int
	for src, cents := range bySource {
		if taxed[src] {
			taxableCents += cents
		}
	}
	return (taxableCents*rateBP + 5000) / 10000
}

// FinancesGET handles GET /api/v1/finances/summary?month=YYYY-MM
//
// One request drives the whole page. The month's sessions and shopping rows
// travel with the totals because the resource list endpoints filter by equality
// only — a month is not expressible as ?filter=col:value — and the page wants
// the numbers and the rows in the same paint anyway.
func FinancesGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := calendar.Now()
		month := r.URL.Query().Get("month")
		if month == "" {
			month = now.Format("2006-01")
		}
		if _, err := time.Parse("2006-01", month); err != nil {
			jsonError(w, "month must be YYYY-MM", 422)
			return
		}
		nowStamp := now.Format("2006-01-02 15:04:05")

		sessions := models.NewTrainingSessionModel(readDB, writeDB, appCache)
		expenses := models.NewExpenseModel(readDB, writeDB, appCache)
		subs := models.NewSubscriptionModel(readDB, writeDB, appCache)
		syncs := models.NewCalendarSyncModel(readDB, writeDB, appCache)

		income, err := sessions.MonthIncome(month, nowStamp)
		if err != nil {
			jsonError(w, "failed to load income", 500)
			return
		}
		bought, committed, err := expenses.MonthTotals(month)
		if err != nil {
			jsonError(w, "failed to load spending", 500)
			return
		}
		subsCents, err := subs.MonthlyEquivalentFor(month)
		if err != nil {
			jsonError(w, "failed to load subscriptions", 500)
			return
		}

		allTimeIncome, err := sessions.AllTimeEarned(nowStamp)
		if err != nil {
			jsonError(w, "failed to load totals", 500)
			return
		}
		allTimeBySource, err := sessions.AllTimeEarnedBySource(nowStamp)
		if err != nil {
			jsonError(w, "failed to load totals", 500)
			return
		}
		allTimeBought, err := expenses.AllTimeBought()
		if err != nil {
			jsonError(w, "failed to load totals", 500)
			return
		}
		// "All time" must not move when the user browses a past month, so this
		// is anchored to the current month (like AllTimeEarned/AllTimeBought
		// above), not the browsed `month` — passing `month` here understates
		// all-time subscription spend for every past month browsed, which
		// silently inflates all_time.net_cents.
		allTimeSubs, err := subs.TotalThrough(now.Format("2006-01"))
		if err != nil {
			jsonError(w, "failed to load totals", 500)
			return
		}

		reviewCount, err := sessions.NeedsReviewCount()
		if err != nil {
			jsonError(w, "failed to load review queue", 500)
			return
		}
		unmatched, err := sessions.UnmatchedNames(20)
		if err != nil {
			jsonError(w, "failed to load review queue", 500)
			return
		}
		monthSessions, err := sessions.ForMonth(month)
		if err != nil {
			jsonError(w, "failed to load sessions", 500)
			return
		}
		monthExpenses, err := expenses.ForMonth(month)
		if err != nil {
			jsonError(w, "failed to load shopping list", 500)
			return
		}
		lastSync, err := syncs.Latest()
		if err != nil {
			jsonError(w, "failed to load sync log", 500)
			return
		}

		// Tax is money that is gone the moment it's earned, on gym (source
		// 'wl' by default) income only -- computed from the same earned/
		// projected/all-time source splits income already uses, so a change
		// to FINANCE_TAX_SOURCES or the earned/projected filters never lets
		// tax and income disagree about which sessions count.
		taxRateBP := financeTaxRateBP()
		taxSources := financeTaxSources()
		taxCents := financeTaxCents(income.EarnedBySource, taxSources, taxRateBP)
		projectedTaxCents := financeTaxCents(income.ProjectedBySource, taxSources, taxRateBP)
		allTimeTax := financeTaxCents(allTimeBySource, taxSources, taxRateBP)

		// Planned shopping never reduces net — it is reported separately, and
		// net_after_committed shows what the month would look like if every
		// planned item were bought today.
		net := income.EarnedCents - taxCents - subsCents - bought

		// The same two figures against projected rather than earned income:
		// "how does the month end up if every session already booked happens?"
		// Earned-based net is the headline because it is money that actually
		// exists; these answer the forward-looking question behind a purchase.
		projectedNet := income.ProjectedCents - projectedTaxCents - subsCents - bought
		allTimeSpend := allTimeBought + allTimeSubs + allTimeTax

		jsonOK(w, map[string]any{
			"month": month,
			"income": map[string]any{
				"earned_cents":    income.EarnedCents,
				"projected_cents": income.ProjectedCents,
				"session_count":   income.SessionCount,
				"by_source":       income.EarnedBySource, // earned only; projected is whole-month
			},
			"spending": map[string]any{
				"subscriptions_cents":      subsCents,
				"shopping_bought_cents":    bought,
				"shopping_committed_cents": committed,
			},
			"tax_cents":           taxCents,
			"projected_tax_cents": projectedTaxCents,
			"tax_rate_bp":         taxRateBP,
			// Which sources are taxed, so the page never has to hardcode it.
			// The chart accrues tax per day from the session rows, and without
			// this it would keep taxing only 'wl' after FINANCE_TAX_SOURCES
			// changed -- the ledger would move and the chart would not.
			"tax_sources":                         taxSourceList(taxSources),
			"net_cents":                           net,
			"net_after_committed_cents":           net - committed,
			"projected_net_cents":                 projectedNet,
			"projected_net_after_committed_cents": projectedNet - committed,
			"all_time": map[string]any{
				"income_cents": allTimeIncome,
				"spend_cents":  allTimeSpend,
				"net_cents":    allTimeIncome - allTimeSpend,
			},
			"needs_review_count": reviewCount,
			"unmatched_names":    unmatched,
			"last_sync":          lastSync,
			"sessions":           monthSessions,
			"expenses":           monthExpenses,
		})
	}
}

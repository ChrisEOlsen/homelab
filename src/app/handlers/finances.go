package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"gova/app/cache"
	"gova/app/calendar"
	"gova/app/models"
)

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

		// Planned shopping never reduces net — it is reported separately, and
		// net_after_committed shows what the month would look like if every
		// planned item were bought today.
		net := income.EarnedCents - subsCents - bought
		allTimeSpend := allTimeBought + allTimeSubs

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
			"net_cents":                 net,
			"net_after_committed_cents": net - committed,
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

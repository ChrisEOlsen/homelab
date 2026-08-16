package handlers

import (
	"database/sql"
	"net/http"

	"gova/app/cache"
	"gova/app/calendar"
)

// CalendarSyncPOST handles POST /api/v1/calendar/sync
//
// An unreachable or empty feed is a *result*, not an endpoint failure: the
// response stays 200 with ok:true and carries a Result whose own ok flag is
// false, so the page's Sync button always gets something to display. A 500 here
// would tell the user their app is broken when it is the scraper that is.
func CalendarSyncPOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		svc := calendar.NewFromDB(readDB, writeDB, appCache)
		jsonOK(w, svc.Run(r.Context()))
	}
}

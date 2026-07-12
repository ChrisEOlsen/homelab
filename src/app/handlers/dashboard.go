package handlers

import (
	"database/sql"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func DashboardGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		shortcutModel := models.NewShortcutModel(readDB, writeDB, appCache)
		focusModel := models.NewFocusModel(readDB, writeDB, appCache)
		reminderModel := models.NewReminderModel(readDB, writeDB, appCache)

		shortcuts, err := shortcutModel.GetAll()
		if err != nil {
			jsonError(w, "failed to load", 500)
			return
		}
		focuses, err := focusModel.GetAll()
		if err != nil {
			jsonError(w, "failed to load", 500)
			return
		}
		reminders, err := reminderModel.GetUpcoming(5)
		if err != nil {
			jsonError(w, "failed to load", 500)
			return
		}
		jsonOK(w, map[string]any{"shortcuts": shortcuts, "focuses": focuses, "reminders": reminders})
	}
}

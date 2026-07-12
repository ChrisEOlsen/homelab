package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func RemindersCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Title          string `json:"title"`
			RemindAt       string `json:"remind_at"`
			RecurrenceType string `json:"recurrence_type"`
			RecurrenceDays string `json:"recurrence_days"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.RemindAt == "" {
			jsonError(w, "title and remind_at are required", 400)
			return
		}
		if body.RecurrenceType == "" {
			body.RecurrenceType = "none"
		}
		model := models.NewReminderModel(readDB, writeDB, appCache)
		id, err := model.Create(body.Title, body.RemindAt, body.RecurrenceType, body.RecurrenceDays, true)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

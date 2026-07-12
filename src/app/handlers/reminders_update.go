package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gova/app/cache"
	"gova/app/models"
)

func RemindersUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			jsonError(w, "invalid id", 400)
			return
		}
		var body struct {
			Title          string `json:"title"`
			RemindAt       string `json:"remind_at"`
			RecurrenceType string `json:"recurrence_type"`
			RecurrenceDays string `json:"recurrence_days"`
			IsActive       bool   `json:"is_active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.RemindAt == "" {
			jsonError(w, "title and remind_at are required", 400)
			return
		}
		model := models.NewReminderModel(readDB, writeDB, appCache)
		if err := model.Update(id, body.Title, body.RemindAt, body.RecurrenceType, body.RecurrenceDays, body.IsActive); err != nil {
			jsonError(w, "failed to update", 500)
			return
		}
		jsonOK(w, nil)
	}
}

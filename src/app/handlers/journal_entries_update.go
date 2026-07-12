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

func JournalEntriesUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			jsonError(w, "invalid id", 400)
			return
		}
		var body struct {
			Title     string `json:"title"`
			Content   string `json:"content"`
			Mood      string `json:"mood"`
			EntryDate string `json:"entry_date"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.EntryDate == "" {
			jsonError(w, "entry_date is required", 400)
			return
		}
		if body.Mood == "" {
			body.Mood = "neutral"
		}
		model := models.NewJournalEntryModel(readDB, writeDB, appCache)
		if err := model.Update(id, body.Title, body.Content, body.Mood, body.EntryDate); err != nil {
			jsonError(w, "failed to update", 500)
			return
		}
		jsonOK(w, nil)
	}
}

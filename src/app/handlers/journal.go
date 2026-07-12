package handlers

import (
	"database/sql"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func JournalGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		model := models.NewJournalEntryModel(readDB, writeDB, appCache)
		items, err := model.GetAll()
		if err != nil {
			jsonError(w, "failed to load", 500)
			return
		}
		jsonOK(w, items)
	}
}

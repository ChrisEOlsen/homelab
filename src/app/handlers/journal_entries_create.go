package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"gova/app/cache"
	"gova/app/models"
)

func JournalEntriesCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		model := models.NewJournalEntryModel(readDB, writeDB, appCache)
		id, err := model.Create("", "", "neutral", time.Now().Format("2006-01-02"))
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

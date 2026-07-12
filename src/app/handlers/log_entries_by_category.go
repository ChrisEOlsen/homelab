package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gova/app/cache"
	"gova/app/models"
)

func LogEntriesByCategoryGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		categoryID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			jsonError(w, "invalid id", 400)
			return
		}
		model := models.NewLogEntryModel(readDB, writeDB, appCache)
		items, err := model.GetByCategory(categoryID)
		if err != nil {
			jsonError(w, "failed to load", 500)
			return
		}
		jsonOK(w, items)
	}
}

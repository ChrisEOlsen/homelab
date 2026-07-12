package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gova/app/cache"
	"gova/app/models"
)

func TodosTogglePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			jsonError(w, "invalid id", 400)
			return
		}
		model := models.NewTodoModel(readDB, writeDB, appCache)
		if err := model.Toggle(id); err != nil {
			jsonError(w, "failed to update", 500)
			return
		}
		jsonOK(w, nil)
	}
}

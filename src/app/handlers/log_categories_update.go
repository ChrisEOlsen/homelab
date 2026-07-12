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

func LogCategoriesUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			jsonError(w, "invalid id", 400)
			return
		}
		var body struct {
			Title  string     `json:"title"`
			Fields []logField `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" {
			jsonError(w, "title is required", 400)
			return
		}
		schemaBytes, err := json.Marshal(body.Fields)
		if err != nil {
			jsonError(w, "invalid fields", 400)
			return
		}
		model := models.NewLogCategoryModel(readDB, writeDB, appCache)
		if err := model.Update(id, body.Title, string(schemaBytes)); err != nil {
			jsonError(w, "failed to update", 500)
			return
		}
		jsonOK(w, nil)
	}
}

package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

type logField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func LogCategoriesCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		id, err := model.Create(body.Title, string(schemaBytes))
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

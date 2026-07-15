package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func CodexEntriesCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Title       string `json:"title"`
			Language    string `json:"language"`
			Code        string `json:"code"`
			Description string `json:"description"`
			Folder      string `json:"folder"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.Code == "" {
			jsonError(w, "title and code are required", 400)
			return
		}
		if body.Language == "" {
			body.Language = "c"
		}
		model := models.NewCodexEntryModel(readDB, writeDB, appCache)
		id, err := model.Create(body.Title, body.Language, body.Code, body.Description, body.Folder)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

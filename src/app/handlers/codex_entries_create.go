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
			Tags        string `json:"tags"`
			Description string `json:"description"`
			BundleID    string `json:"bundle_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.Code == "" {
			jsonError(w, "title and code are required", 400)
			return
		}
		if body.Language == "" {
			body.Language = "c"
		}
		model := models.NewCodexEntryModel(readDB, writeDB, appCache)
		id, err := model.Create(body.Title, body.Language, body.Code, body.Tags, body.Description, body.BundleID)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func CodexFoldersDeletePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Path == "" {
			jsonError(w, "path is required", 400)
			return
		}
		model := models.NewCodexEntryModel(readDB, writeDB, appCache)
		if err := model.DeleteFolder(body.Path); err != nil {
			jsonError(w, "failed to delete folder", 500)
			return
		}
		jsonOK(w, nil)
	}
}

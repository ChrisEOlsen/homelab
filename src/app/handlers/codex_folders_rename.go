package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func CodexFoldersRenamePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			OldPath string `json:"old_path"`
			NewPath string `json:"new_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.OldPath == "" || body.NewPath == "" {
			jsonError(w, "old_path and new_path are required", 400)
			return
		}
		model := models.NewCodexEntryModel(readDB, writeDB, appCache)
		if err := model.RenameFolder(body.OldPath, body.NewPath); err != nil {
			jsonError(w, "failed to rename folder", 500)
			return
		}
		jsonOK(w, nil)
	}
}

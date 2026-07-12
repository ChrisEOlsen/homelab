package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func LogEntriesCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			CategoryID int64                  `json:"category_id"`
			Data       map[string]interface{} `json:"data"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.CategoryID == 0 {
			jsonError(w, "category_id is required", 400)
			return
		}
		dataBytes, err := json.Marshal(body.Data)
		if err != nil {
			jsonError(w, "invalid data", 400)
			return
		}
		model := models.NewLogEntryModel(readDB, writeDB, appCache)
		id, err := model.Create(body.CategoryID, string(dataBytes))
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

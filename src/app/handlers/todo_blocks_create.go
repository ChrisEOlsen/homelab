package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func TodoBlocksCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			TodoID int64 `json:"todo_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TodoID == 0 {
			jsonError(w, "todo_id is required", 400)
			return
		}
		model := models.NewTodoBlockModel(readDB, writeDB, appCache)
		id, err := model.Create(body.TodoID, "New Section", "", 99)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

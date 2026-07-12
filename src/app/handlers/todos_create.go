package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func TodosCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ListID      int64  `json:"list_id"`
			Title       string `json:"title"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.ListID == 0 {
			jsonError(w, "list_id and title are required", 400)
			return
		}
		model := models.NewTodoModel(readDB, writeDB, appCache)
		id, err := model.Create(body.ListID, body.Title, false, body.Description, 0)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

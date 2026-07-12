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

func TodosUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			jsonError(w, "invalid id", 400)
			return
		}
		var body struct {
			ListID      int64  `json:"list_id"`
			Title       string `json:"title"`
			IsDone      bool   `json:"is_done"`
			Description string `json:"description"`
			SortOrder   int    `json:"sort_order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" {
			jsonError(w, "title is required", 400)
			return
		}
		model := models.NewTodoModel(readDB, writeDB, appCache)
		if err := model.Update(id, body.ListID, body.Title, body.IsDone, body.Description, body.SortOrder); err != nil {
			jsonError(w, "failed to update", 500)
			return
		}
		jsonOK(w, nil)
	}
}

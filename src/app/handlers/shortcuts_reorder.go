package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func ShortcutsReorderPUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Order []int64 `json:"order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Order) == 0 {
			jsonError(w, "order is required", 400)
			return
		}
		model := models.NewShortcutModel(readDB, writeDB, appCache)
		for i, id := range body.Order {
			if err := model.UpdateSortOrder(id, i); err != nil {
				jsonError(w, "failed to reorder", 500)
				return
			}
		}
		jsonOK(w, nil)
	}
}

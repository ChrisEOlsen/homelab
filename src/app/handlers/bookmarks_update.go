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

func BookmarksUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			jsonError(w, "invalid id", 400)
			return
		}
		var body struct {
			CategoryID  int64  `json:"category_id"`
			Title       string `json:"title"`
			Url         string `json:"url"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.Url == "" || body.CategoryID == 0 {
			jsonError(w, "category_id, title and url are required", 400)
			return
		}
		if !strings_hasScheme(body.Url) {
			body.Url = "https://" + body.Url
		}
		model := models.NewBookmarkModel(readDB, writeDB, appCache)
		if err := model.Update(id, body.CategoryID, body.Title, body.Url, body.Description); err != nil {
			jsonError(w, "failed to update", 500)
			return
		}
		jsonOK(w, nil)
	}
}

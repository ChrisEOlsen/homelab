package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func BookmarksCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		id, err := model.Create(body.CategoryID, body.Title, body.Url, body.Description)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

func strings_hasScheme(url string) bool {
	return len(url) >= 7 && (url[:7] == "http://" || (len(url) >= 8 && url[:8] == "https://") || (len(url) >= 6 && url[:6] == "ftp://"))
}

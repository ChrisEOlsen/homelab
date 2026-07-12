package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"gova/app/cache"
	"gova/app/models"
)

func ShortcutsCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Title string `json:"title"`
			Url   string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.Url == "" {
			jsonError(w, "title and url are required", 400)
			return
		}
		if !strings.HasPrefix(body.Url, "http://") && !strings.HasPrefix(body.Url, "https://") && !strings.HasPrefix(body.Url, "ftp://") {
			body.Url = "https://" + body.Url
		}
		model := models.NewShortcutModel(readDB, writeDB, appCache)
		id, err := model.Create(body.Title, body.Url)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

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

func CodexEntriesUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			jsonError(w, "invalid id", 400)
			return
		}
		var body struct {
			Title       string `json:"title"`
			Language    string `json:"language"`
			Code        string `json:"code"`
			Tags        string `json:"tags"`
			Description string `json:"description"`
			BundleID    string `json:"bundle_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.Code == "" {
			jsonError(w, "title and code are required", 400)
			return
		}
		model := models.NewCodexEntryModel(readDB, writeDB, appCache)
		if err := model.Update(id, body.Title, body.Language, body.Code, body.Tags, body.Description, body.BundleID); err != nil {
			jsonError(w, "failed to update", 500)
			return
		}
		jsonOK(w, nil)
	}
}

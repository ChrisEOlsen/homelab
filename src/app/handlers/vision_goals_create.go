package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func VisionGoalsCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			CategoryID int64  `json:"category_id"`
			Title      string `json:"title"`
			TargetYear int64  `json:"target_year"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.CategoryID == 0 {
			jsonError(w, "category_id and title are required", 400)
			return
		}
		model := models.NewVisionGoalModel(readDB, writeDB, appCache)
		id, err := model.Create(body.CategoryID, body.Title, body.TargetYear)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

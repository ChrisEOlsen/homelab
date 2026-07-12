package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func VisionMilestonesCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			GoalID int64  `json:"goal_id"`
			Title  string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.GoalID == 0 {
			jsonError(w, "goal_id and title are required", 400)
			return
		}
		model := models.NewVisionMilestoneModel(readDB, writeDB, appCache)
		id, err := model.Create(body.GoalID, body.Title, false)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

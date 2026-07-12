package handlers

import (
	"database/sql"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func VisionBoardGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		catModel := models.NewVisionCategoryModel(readDB, writeDB, appCache)
		goalModel := models.NewVisionGoalModel(readDB, writeDB, appCache)
		milestoneModel := models.NewVisionMilestoneModel(readDB, writeDB, appCache)

		categories, err := catModel.GetAll()
		if err != nil {
			jsonError(w, "failed to load", 500)
			return
		}
		goals, err := goalModel.GetAll()
		if err != nil {
			jsonError(w, "failed to load", 500)
			return
		}
		milestones, err := milestoneModel.GetAll()
		if err != nil {
			jsonError(w, "failed to load", 500)
			return
		}
		jsonOK(w, map[string]any{"categories": categories, "goals": goals, "milestones": milestones})
	}
}

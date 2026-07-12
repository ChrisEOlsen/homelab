package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gova/app/cache"
	"gova/app/models"
)

func VisionCategoriesDeleteDELETE(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			jsonError(w, "invalid id", 400)
			return
		}
		model := models.NewVisionCategoryModel(readDB, writeDB, appCache)
		if err := model.Delete(id); err != nil {
			jsonError(w, "failed to delete", 500)
			return
		}
		// The DB-level ON DELETE CASCADE removes this category's goals, and in
		// turn those goals' milestones, so both caches must be invalidated here
		// too — otherwise stale (deleted) goals/milestones would keep being
		// served for up to the cache TTL.
		appCache.Bust("vision_goals:")
		appCache.Bust("vision_milestones:")
		jsonOK(w, nil)
	}
}

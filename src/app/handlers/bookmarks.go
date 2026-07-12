package handlers

import (
	"database/sql"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func BookmarksGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		catModel := models.NewBookmarkCategoryModel(readDB, writeDB, appCache)
		bmModel := models.NewBookmarkModel(readDB, writeDB, appCache)
		categories, err := catModel.GetAll()
		if err != nil {
			jsonError(w, "failed to load", 500)
			return
		}
		bookmarks, err := bmModel.GetAll()
		if err != nil {
			jsonError(w, "failed to load", 500)
			return
		}
		jsonOK(w, map[string]any{"categories": categories, "bookmarks": bookmarks})
	}
}

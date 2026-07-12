package handlers

import (
	"database/sql"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func TodosGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		listModel := models.NewTodoListModel(readDB, writeDB, appCache)
		todoModel := models.NewTodoModel(readDB, writeDB, appCache)
		lists, err := listModel.GetAll()
		if err != nil {
			jsonError(w, "failed to load", 500)
			return
		}
		todos, err := todoModel.GetAll()
		if err != nil {
			jsonError(w, "failed to load", 500)
			return
		}
		jsonOK(w, map[string]any{"lists": lists, "todos": todos})
	}
}

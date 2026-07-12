package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"gova/app/cache"
	"gova/app/models"
)

func TodoDetailsGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		todoID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
		if err != nil {
			jsonError(w, "invalid id", 400)
			return
		}
		todoModel := models.NewTodoModel(readDB, writeDB, appCache)
		todo, err := todoModel.Find(todoID)
		if err != nil {
			jsonError(w, "not found", 404)
			return
		}
		subtaskModel := models.NewSubtaskModel(readDB, writeDB, appCache)
		subtasks, err := subtaskModel.GetByTodo(todoID)
		if err != nil {
			jsonError(w, "failed to load", 500)
			return
		}
		blockModel := models.NewTodoBlockModel(readDB, writeDB, appCache)
		blocks, err := blockModel.GetByTodo(todoID)
		if err != nil {
			jsonError(w, "failed to load", 500)
			return
		}
		jsonOK(w, map[string]any{"todo": todo, "subtasks": subtasks, "blocks": blocks})
	}
}

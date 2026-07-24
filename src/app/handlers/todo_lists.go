package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

// TodoListListGET handles GET /api/v1/todo_lists
func TodoListListGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset, opts := listQuery(r)
		model := models.NewTodoListModel(readDB, writeDB, appCache)
		items, total, err := model.GetPage(limit, offset, opts)
		if err != nil {
			if errors.Is(err, models.ErrInvalidQuery) {
				jsonError(w, "invalid sort/filter column", 422)
				return
			}
			jsonError(w, "failed to load", 500)
			return
		}
		jsonList(w, items, Meta{Limit: limit, Offset: offset, Total: total})
	}
}

// TodoListDetailGET handles GET /api/v1/todo_lists/{id}
func TodoListDetailGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewTodoListModel(readDB, writeDB, appCache)
		item, err := model.Find(id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				jsonError(w, "not found", 404)
				return
			}
			jsonError(w, "failed to load", 500)
			return
		}
		jsonOK(w, item)
	}
}

// TodoListCreatePOST handles POST /api/v1/todo_lists
func TodoListCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" {
			jsonError(w, "title is required", 422)
			return
		}
		model := models.NewTodoListModel(readDB, writeDB, appCache)
		id, err := model.Create(body.Title, 0)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// TodoListUpdatePUT handles PUT /api/v1/todo_lists/{id}
func TodoListUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		var body struct {
			Title     string `json:"title"`
			SortOrder int    `json:"sort_order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" {
			jsonError(w, "title is required", 422)
			return
		}
		model := models.NewTodoListModel(readDB, writeDB, appCache)
		if err := model.Update(id, body.Title, body.SortOrder); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				jsonError(w, "not found", 404)
				return
			}
			jsonError(w, "failed to update", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// TodoListDeleteDELETE handles DELETE /api/v1/todo_lists/{id}
func TodoListDeleteDELETE(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewTodoListModel(readDB, writeDB, appCache)
		if err := model.Delete(id); err != nil {
			jsonError(w, "failed to delete", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

// TodoListClearCompletedPOST handles POST /api/v1/todo_lists/{id}/clear_completed
func TodoListClearCompletedPOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewTodoModel(readDB, writeDB, appCache)
		if err := model.ClearCompleted(id); err != nil {
			jsonError(w, "failed to clear completed", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

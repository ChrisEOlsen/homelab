package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

// TodoBlockListGET handles GET /api/v1/todo_blocks
// Filter by todo with ?filter=todo_id:<id>.
func TodoBlockListGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset, opts := listQuery(r)
		model := models.NewTodoBlockModel(readDB, writeDB, appCache)
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

// TodoBlockDetailGET handles GET /api/v1/todo_blocks/{id}
func TodoBlockDetailGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewTodoBlockModel(readDB, writeDB, appCache)
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

// TodoBlockCreatePOST handles POST /api/v1/todo_blocks
func TodoBlockCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			TodoID    int64  `json:"todo_id"`
			Header    string `json:"header"`
			Content   string `json:"content"`
			SortOrder int64  `json:"sort_order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.TodoID == 0 {
			jsonError(w, "todo_id is required", 422)
			return
		}
		if body.Header == "" {
			body.Header = "New Section"
		}
		model := models.NewTodoBlockModel(readDB, writeDB, appCache)
		id, err := model.Create(body.TodoID, body.Header, body.Content, body.SortOrder)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// TodoBlockUpdatePUT handles PUT /api/v1/todo_blocks/{id}
func TodoBlockUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		var body struct {
			Header    string `json:"header"`
			Content   string `json:"content"`
			SortOrder int64  `json:"sort_order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "invalid request body", 422)
			return
		}
		model := models.NewTodoBlockModel(readDB, writeDB, appCache)
		if err := model.Update(id, body.Header, body.Content, body.SortOrder); err != nil {
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

// TodoBlockDeleteDELETE handles DELETE /api/v1/todo_blocks/{id}
func TodoBlockDeleteDELETE(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewTodoBlockModel(readDB, writeDB, appCache)
		if err := model.Delete(id); err != nil {
			jsonError(w, "failed to delete", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

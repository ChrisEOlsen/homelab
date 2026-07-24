package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

// TodoListGET handles GET /api/v1/todos
// Filter by list with ?filter=list_id:<id>.
func TodoListGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset, opts := listQuery(r)
		model := models.NewTodoModel(readDB, writeDB, appCache)
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

// TodoDetailGET handles GET /api/v1/todos/{id}
func TodoDetailGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewTodoModel(readDB, writeDB, appCache)
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

// TodoCreatePOST handles POST /api/v1/todos
func TodoCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ListID      int64   `json:"list_id"`
			Title       string  `json:"title"`
			Description *string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.ListID == 0 {
			jsonError(w, "list_id and title are required", 422)
			return
		}
		model := models.NewTodoModel(readDB, writeDB, appCache)
		id, err := model.Create(body.ListID, body.Title, false, body.Description, 0)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// TodoUpdatePUT handles PUT /api/v1/todos/{id}
func TodoUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		var body struct {
			ListID      int64   `json:"list_id"`
			Title       string  `json:"title"`
			IsDone      bool    `json:"is_done"`
			Description *string `json:"description"`
			SortOrder   int     `json:"sort_order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" {
			jsonError(w, "title is required", 422)
			return
		}
		model := models.NewTodoModel(readDB, writeDB, appCache)
		if err := model.Update(id, body.ListID, body.Title, body.IsDone, body.Description, body.SortOrder); err != nil {
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

// TodoDeleteDELETE handles DELETE /api/v1/todos/{id}
func TodoDeleteDELETE(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewTodoModel(readDB, writeDB, appCache)
		if err := model.Delete(id); err != nil {
			jsonError(w, "failed to delete", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

// TodoTogglePOST handles POST /api/v1/todos/{id}/toggle
func TodoTogglePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewTodoModel(readDB, writeDB, appCache)
		if err := model.Toggle(id); err != nil {
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

// TodoReorderPUT handles PUT /api/v1/todos/reorder. The body is the ordered id
// list; each todo's sort_order is set to its index.
func TodoReorderPUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Order []int64 `json:"order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Order) == 0 {
			jsonError(w, "order is required", 422)
			return
		}
		model := models.NewTodoModel(readDB, writeDB, appCache)
		for i, id := range body.Order {
			if err := model.UpdateSortOrder(id, i); err != nil {
				jsonError(w, "failed to reorder", 500)
				return
			}
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

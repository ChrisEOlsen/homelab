package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

// ShortcutListGET handles GET /api/v1/shortcuts
func ShortcutListGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset, opts := listQuery(r)
		model := models.NewShortcutModel(readDB, writeDB, appCache)
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

// ShortcutDetailGET handles GET /api/v1/shortcuts/{id}
func ShortcutDetailGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewShortcutModel(readDB, writeDB, appCache)
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

// ShortcutCreatePOST handles POST /api/v1/shortcuts
func ShortcutCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Title string `json:"title"`
			Url   string `json:"url"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.Url == "" {
			jsonError(w, "title and url are required", 422)
			return
		}
		body.Url = normalizeURL(body.Url)
		model := models.NewShortcutModel(readDB, writeDB, appCache)
		id, err := model.Create(body.Title, body.Url, 0)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// ShortcutDeleteDELETE handles DELETE /api/v1/shortcuts/{id}
func ShortcutDeleteDELETE(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewShortcutModel(readDB, writeDB, appCache)
		if err := model.Delete(id); err != nil {
			jsonError(w, "failed to delete", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

// ShortcutReorderPUT handles PUT /api/v1/shortcuts/reorder
func ShortcutReorderPUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Order []int64 `json:"order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Order) == 0 {
			jsonError(w, "order is required", 422)
			return
		}
		model := models.NewShortcutModel(readDB, writeDB, appCache)
		for i, id := range body.Order {
			if err := model.UpdateSortOrder(id, i); err != nil {
				jsonError(w, "failed to reorder", 500)
				return
			}
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

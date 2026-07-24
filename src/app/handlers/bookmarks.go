package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"gova/app/cache"
	"gova/app/models"
)

// normalizeURL prepends https:// when the value carries no explicit scheme, so
// a bookmark saved as "example.com" still links out correctly.
func normalizeURL(url string) string {
	lower := strings.ToLower(url)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "ftp://") {
		return url
	}
	return "https://" + url
}

// BookmarkListGET handles GET /api/v1/bookmarks
func BookmarkListGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset, opts := listQuery(r)
		model := models.NewBookmarkModel(readDB, writeDB, appCache)
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

// BookmarkDetailGET handles GET /api/v1/bookmarks/{id}
func BookmarkDetailGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewBookmarkModel(readDB, writeDB, appCache)
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

// BookmarkCreatePOST handles POST /api/v1/bookmarks
func BookmarkCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			CategoryID  int64   `json:"category_id"`
			Title       string  `json:"title"`
			Url         string  `json:"url"`
			Description *string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.Url == "" || body.CategoryID == 0 {
			jsonError(w, "category_id, title and url are required", 422)
			return
		}
		body.Url = normalizeURL(body.Url)
		model := models.NewBookmarkModel(readDB, writeDB, appCache)
		id, err := model.Create(body.CategoryID, body.Title, body.Url, body.Description)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// BookmarkUpdatePUT handles PUT /api/v1/bookmarks/{id}
func BookmarkUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		var body struct {
			CategoryID  int64   `json:"category_id"`
			Title       string  `json:"title"`
			Url         string  `json:"url"`
			Description *string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.Url == "" || body.CategoryID == 0 {
			jsonError(w, "category_id, title and url are required", 422)
			return
		}
		body.Url = normalizeURL(body.Url)
		model := models.NewBookmarkModel(readDB, writeDB, appCache)
		if err := model.Update(id, body.CategoryID, body.Title, body.Url, body.Description); err != nil {
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

// BookmarkDeleteDELETE handles DELETE /api/v1/bookmarks/{id}
func BookmarkDeleteDELETE(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewBookmarkModel(readDB, writeDB, appCache)
		if err := model.Delete(id); err != nil {
			jsonError(w, "failed to delete", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

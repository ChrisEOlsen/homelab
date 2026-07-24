package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

// logField is one field definition in a log category's schema. The set is
// stored serialized in log_categories.schema_def.
type logField struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// LogCategoryListGET handles GET /api/v1/log_categories
func LogCategoryListGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset, opts := listQuery(r)
		model := models.NewLogCategoryModel(readDB, writeDB, appCache)
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

// LogCategoryDetailGET handles GET /api/v1/log_categories/{id}
func LogCategoryDetailGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewLogCategoryModel(readDB, writeDB, appCache)
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

// LogCategoryCreatePOST handles POST /api/v1/log_categories
func LogCategoryCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Title  string     `json:"title"`
			Fields []logField `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" {
			jsonError(w, "title is required", 422)
			return
		}
		schemaBytes, err := json.Marshal(body.Fields)
		if err != nil {
			jsonError(w, "invalid fields", 422)
			return
		}
		model := models.NewLogCategoryModel(readDB, writeDB, appCache)
		id, err := model.Create(body.Title, string(schemaBytes))
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// LogCategoryUpdatePUT handles PUT /api/v1/log_categories/{id}
func LogCategoryUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		var body struct {
			Title  string     `json:"title"`
			Fields []logField `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" {
			jsonError(w, "title is required", 422)
			return
		}
		schemaBytes, err := json.Marshal(body.Fields)
		if err != nil {
			jsonError(w, "invalid fields", 422)
			return
		}
		model := models.NewLogCategoryModel(readDB, writeDB, appCache)
		if err := model.Update(id, body.Title, string(schemaBytes)); err != nil {
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

// LogCategoryDeleteDELETE handles DELETE /api/v1/log_categories/{id}
func LogCategoryDeleteDELETE(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewLogCategoryModel(readDB, writeDB, appCache)
		if err := model.Delete(id); err != nil {
			jsonError(w, "failed to delete", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

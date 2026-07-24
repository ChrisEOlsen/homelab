package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

// CodexEntryListGET handles GET /api/v1/codex_entries
func CodexEntryListGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset, opts := listQuery(r)
		model := models.NewCodexEntryModel(readDB, writeDB, appCache)
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

// CodexEntryDetailGET handles GET /api/v1/codex_entries/{id}
func CodexEntryDetailGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewCodexEntryModel(readDB, writeDB, appCache)
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

// codexBody is the shared request shape for create and update.
type codexBody struct {
	Title       string  `json:"title"`
	Language    *string `json:"language"`
	Code        string  `json:"code"`
	Description *string `json:"description"`
	Folder      string  `json:"folder"`
}

// defaultLanguage returns a non-nil "c" when no language was supplied, matching
// the app's default snippet language.
func (b codexBody) language() *string {
	if b.Language == nil || *b.Language == "" {
		def := "c"
		return &def
	}
	return b.Language
}

// CodexEntryCreatePOST handles POST /api/v1/codex_entries
func CodexEntryCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body codexBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.Code == "" {
			jsonError(w, "title and code are required", 422)
			return
		}
		model := models.NewCodexEntryModel(readDB, writeDB, appCache)
		id, err := model.Create(body.Title, body.language(), body.Code, body.Description, body.Folder)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// CodexEntryUpdatePUT handles PUT /api/v1/codex_entries/{id}
func CodexEntryUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		var body codexBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.Code == "" {
			jsonError(w, "title and code are required", 422)
			return
		}
		model := models.NewCodexEntryModel(readDB, writeDB, appCache)
		if err := model.Update(id, body.Title, body.language(), body.Code, body.Description, body.Folder); err != nil {
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

// CodexEntryDeleteDELETE handles DELETE /api/v1/codex_entries/{id}
func CodexEntryDeleteDELETE(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewCodexEntryModel(readDB, writeDB, appCache)
		if err := model.Delete(id); err != nil {
			jsonError(w, "failed to delete", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

// CodexFolderRenamePOST handles POST /api/v1/codex_folders/rename
func CodexFolderRenamePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			OldPath string `json:"old_path"`
			NewPath string `json:"new_path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.OldPath == "" || body.NewPath == "" {
			jsonError(w, "old_path and new_path are required", 422)
			return
		}
		model := models.NewCodexEntryModel(readDB, writeDB, appCache)
		if err := model.RenameFolder(body.OldPath, body.NewPath); err != nil {
			jsonError(w, "failed to rename folder", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

// CodexFolderDeletePOST handles POST /api/v1/codex_folders/delete
func CodexFolderDeletePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Path == "" {
			jsonError(w, "path is required", 422)
			return
		}
		model := models.NewCodexEntryModel(readDB, writeDB, appCache)
		if err := model.DeleteFolder(body.Path); err != nil {
			jsonError(w, "failed to delete folder", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

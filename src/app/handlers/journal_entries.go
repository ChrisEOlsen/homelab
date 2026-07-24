package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"gova/app/cache"
	"gova/app/models"
)

// JournalEntryListGET handles GET /api/v1/journal_entries
func JournalEntryListGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset, opts := listQuery(r)
		model := models.NewJournalEntryModel(readDB, writeDB, appCache)
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

// JournalEntryDetailGET handles GET /api/v1/journal_entries/{id}
func JournalEntryDetailGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewJournalEntryModel(readDB, writeDB, appCache)
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

// JournalEntryCreatePOST handles POST /api/v1/journal_entries. A create starts a
// blank entry dated today; the client fills it in via the update endpoint.
func JournalEntryCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		model := models.NewJournalEntryModel(readDB, writeDB, appCache)
		id, err := model.Create(nil, "", "neutral", time.Now().Format("2006-01-02"))
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// JournalEntryUpdatePUT handles PUT /api/v1/journal_entries/{id}
func JournalEntryUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		var body struct {
			Title     *string `json:"title"`
			Content   string  `json:"content"`
			Mood      string  `json:"mood"`
			EntryDate string  `json:"entry_date"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.EntryDate == "" {
			jsonError(w, "entry_date is required", 422)
			return
		}
		if body.Mood == "" {
			body.Mood = "neutral"
		}
		model := models.NewJournalEntryModel(readDB, writeDB, appCache)
		if err := model.Update(id, body.Title, body.Content, body.Mood, body.EntryDate); err != nil {
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

// JournalEntryDeleteDELETE handles DELETE /api/v1/journal_entries/{id}
func JournalEntryDeleteDELETE(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewJournalEntryModel(readDB, writeDB, appCache)
		if err := model.Delete(id); err != nil {
			jsonError(w, "failed to delete", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

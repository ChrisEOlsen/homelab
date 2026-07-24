package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

// ReminderListGET handles GET /api/v1/reminders
func ReminderListGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset, opts := listQuery(r)
		model := models.NewReminderModel(readDB, writeDB, appCache)
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

// ReminderDetailGET handles GET /api/v1/reminders/{id}
func ReminderDetailGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewReminderModel(readDB, writeDB, appCache)
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

// ReminderCreatePOST handles POST /api/v1/reminders
func ReminderCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Title          string  `json:"title"`
			RemindAt       string  `json:"remind_at"`
			RecurrenceType string  `json:"recurrence_type"`
			RecurrenceDays *string `json:"recurrence_days"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.RemindAt == "" {
			jsonError(w, "title and remind_at are required", 422)
			return
		}
		if body.RecurrenceType == "" {
			body.RecurrenceType = "none"
		}
		model := models.NewReminderModel(readDB, writeDB, appCache)
		id, err := model.Create(body.Title, body.RemindAt, body.RecurrenceType, body.RecurrenceDays, true)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// ReminderUpdatePUT handles PUT /api/v1/reminders/{id}
func ReminderUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		var body struct {
			Title          string  `json:"title"`
			RemindAt       string  `json:"remind_at"`
			RecurrenceType string  `json:"recurrence_type"`
			RecurrenceDays *string `json:"recurrence_days"`
			IsActive       bool    `json:"is_active"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" || body.RemindAt == "" {
			jsonError(w, "title and remind_at are required", 422)
			return
		}
		if body.RecurrenceType == "" {
			body.RecurrenceType = "none"
		}
		model := models.NewReminderModel(readDB, writeDB, appCache)
		if err := model.Update(id, body.Title, body.RemindAt, body.RecurrenceType, body.RecurrenceDays, body.IsActive); err != nil {
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

// ReminderDeleteDELETE handles DELETE /api/v1/reminders/{id}
func ReminderDeleteDELETE(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewReminderModel(readDB, writeDB, appCache)
		if err := model.Delete(id); err != nil {
			jsonError(w, "failed to delete", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

// ReminderTogglePOST handles POST /api/v1/reminders/{id}/toggle
func ReminderTogglePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewReminderModel(readDB, writeDB, appCache)
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

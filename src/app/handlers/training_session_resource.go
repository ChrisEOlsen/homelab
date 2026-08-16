package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"gova/app/cache"
	"gova/app/models"
)

type training_sessionRequest struct {
	Uid string `json:"uid"`
	Source string `json:"source"`
	ClientName string `json:"client_name"`
	ClientId *int64 `json:"client_id"`
	Service *string `json:"service"`
	SessionDate string `json:"session_date"`
	StartAt string `json:"start_at"`
	EndAt string `json:"end_at"`
	DurationMin int64 `json:"duration_min"`
	AmountCents int64 `json:"amount_cents"`
	RateSource string `json:"rate_source"`
	OverrideCents *int64 `json:"override_cents"`
	Status string `json:"status"`
	NeedsReview bool `json:"needs_review"`
}

func parseTrainingSessionID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// TrainingSessionListGET handles GET /api/v1/training_sessions
// Query: ?limit=<1..200>&offset=<0..>&sort=<[-]col>&filter=<col:value>
func TrainingSessionListGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := queryInt(r, "limit", defaultPageLimit, 1, maxPageLimit)
		offset := queryInt(r, "offset", 0, 0, maxPageOffset)
		opts := models.QueryOpts{Sort: r.URL.Query().Get("sort")}
		if f := r.URL.Query().Get("filter"); f != "" {
			if k, v, ok := strings.Cut(f, ":"); ok {
				opts.FilterField, opts.FilterValue = k, v
			}
		}
		model := models.NewTrainingSessionModel(readDB, writeDB, appCache)
		items, total, err := model.GetPage(limit, offset, opts)
		if err != nil {
			if errors.Is(err, models.ErrInvalidQuery) {
				jsonError(w, "invalid sort/filter column; allowed: id, uid, source, client_name, client_id, service, session_date, start_at, end_at, duration_min, amount_cents, rate_source, override_cents, status, needs_review, created_at", 422)
				return
			}
			jsonError(w, "failed to load", 500)
			return
		}
		jsonList(w, items, Meta{Limit: limit, Offset: offset, Total: total})
	}
}

// TrainingSessionDetailGET handles GET /api/v1/training_sessions/{id}
func TrainingSessionDetailGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseTrainingSessionID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewTrainingSessionModel(readDB, writeDB, appCache)
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

// TrainingSessionCreatePOST handles POST /api/v1/training_sessions
func TrainingSessionCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req training_sessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", 422)
			return
		}
		model := models.NewTrainingSessionModel(readDB, writeDB, appCache)
		id, err := model.Create(req.Uid, req.Source, req.ClientName, req.ClientId, req.Service, req.SessionDate, req.StartAt, req.EndAt, req.DurationMin, req.AmountCents, req.RateSource, req.OverrideCents, req.Status, req.NeedsReview)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// TrainingSessionUpdatePUT handles PUT /api/v1/training_sessions/{id}
func TrainingSessionUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseTrainingSessionID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		var req training_sessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", 422)
			return
		}
		model := models.NewTrainingSessionModel(readDB, writeDB, appCache)
		if err := model.Update(id, req.Uid, req.Source, req.ClientName, req.ClientId, req.Service, req.SessionDate, req.StartAt, req.EndAt, req.DurationMin, req.AmountCents, req.RateSource, req.OverrideCents, req.Status, req.NeedsReview); err != nil {
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

// TrainingSessionDeleteDELETE handles DELETE /api/v1/training_sessions/{id}
func TrainingSessionDeleteDELETE(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseTrainingSessionID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewTrainingSessionModel(readDB, writeDB, appCache)
		if err := model.Delete(id); err != nil {
			jsonError(w, "failed to delete", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

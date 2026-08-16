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

type clientRequest struct {
	Name string `json:"name"`
	MatchName string `json:"match_name"`
	Email *string `json:"email"`
	Phone *string `json:"phone"`
	RateCents int64 `json:"rate_cents"`
	Kind string `json:"kind"`
	IsActive bool `json:"is_active"`
	Notes *string `json:"notes"`
}

func parseClientID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// ClientListGET handles GET /api/v1/clients
// Query: ?limit=<1..200>&offset=<0..>&sort=<[-]col>&filter=<col:value>
func ClientListGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := queryInt(r, "limit", defaultPageLimit, 1, maxPageLimit)
		offset := queryInt(r, "offset", 0, 0, maxPageOffset)
		opts := models.QueryOpts{Sort: r.URL.Query().Get("sort")}
		if f := r.URL.Query().Get("filter"); f != "" {
			if k, v, ok := strings.Cut(f, ":"); ok {
				opts.FilterField, opts.FilterValue = k, v
			}
		}
		model := models.NewClientModel(readDB, writeDB, appCache)
		items, total, err := model.GetPage(limit, offset, opts)
		if err != nil {
			if errors.Is(err, models.ErrInvalidQuery) {
				jsonError(w, "invalid sort/filter column; allowed: id, name, match_name, email, phone, rate_cents, kind, is_active, notes, created_at", 422)
				return
			}
			jsonError(w, "failed to load", 500)
			return
		}
		jsonList(w, items, Meta{Limit: limit, Offset: offset, Total: total})
	}
}

// ClientDetailGET handles GET /api/v1/clients/{id}
func ClientDetailGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseClientID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewClientModel(readDB, writeDB, appCache)
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

// ClientCreatePOST handles POST /api/v1/clients
func ClientCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req clientRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", 422)
			return
		}
		model := models.NewClientModel(readDB, writeDB, appCache)
		id, err := model.Create(req.Name, req.MatchName, req.Email, req.Phone, req.RateCents, req.Kind, req.IsActive, req.Notes)
		if err != nil {
			if isUniqueConstraintErr(err) {
				jsonError(w, "a client with that calendar name already exists", 409)
				return
			}
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// ClientUpdatePUT handles PUT /api/v1/clients/{id}
func ClientUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseClientID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		var req clientRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", 422)
			return
		}
		model := models.NewClientModel(readDB, writeDB, appCache)
		if err := model.Update(id, req.Name, req.MatchName, req.Email, req.Phone, req.RateCents, req.Kind, req.IsActive, req.Notes); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				jsonError(w, "not found", 404)
				return
			}
			if isUniqueConstraintErr(err) {
				jsonError(w, "a client with that calendar name already exists", 409)
				return
			}
			jsonError(w, "failed to update", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// ClientDeleteDELETE handles DELETE /api/v1/clients/{id}
func ClientDeleteDELETE(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseClientID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewClientModel(readDB, writeDB, appCache)
		if err := model.Delete(id); err != nil {
			jsonError(w, "failed to delete", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

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

type rate_ruleRequest struct {
	DurationMin int64 `json:"duration_min"`
	AmountCents int64 `json:"amount_cents"`
	Label *string `json:"label"`
}

func parseRateRuleID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// RateRuleListGET handles GET /api/v1/rate_rules
// Query: ?limit=<1..200>&offset=<0..>&sort=<[-]col>&filter=<col:value>
func RateRuleListGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := queryInt(r, "limit", defaultPageLimit, 1, maxPageLimit)
		offset := queryInt(r, "offset", 0, 0, maxPageOffset)
		opts := models.QueryOpts{Sort: r.URL.Query().Get("sort")}
		if f := r.URL.Query().Get("filter"); f != "" {
			if k, v, ok := strings.Cut(f, ":"); ok {
				opts.FilterField, opts.FilterValue = k, v
			}
		}
		model := models.NewRateRuleModel(readDB, writeDB, appCache)
		items, total, err := model.GetPage(limit, offset, opts)
		if err != nil {
			if errors.Is(err, models.ErrInvalidQuery) {
				jsonError(w, "invalid sort/filter column; allowed: id, duration_min, amount_cents, label, created_at", 422)
				return
			}
			jsonError(w, "failed to load", 500)
			return
		}
		jsonList(w, items, Meta{Limit: limit, Offset: offset, Total: total})
	}
}

// RateRuleDetailGET handles GET /api/v1/rate_rules/{id}
func RateRuleDetailGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseRateRuleID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewRateRuleModel(readDB, writeDB, appCache)
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

// RateRuleCreatePOST handles POST /api/v1/rate_rules
func RateRuleCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req rate_ruleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", 422)
			return
		}
		model := models.NewRateRuleModel(readDB, writeDB, appCache)
		id, err := model.Create(req.DurationMin, req.AmountCents, req.Label)
		if err != nil {
			if isUniqueConstraintErr(err) {
				jsonError(w, "a rate rule for that duration already exists", 409)
				return
			}
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// RateRuleUpdatePUT handles PUT /api/v1/rate_rules/{id}
func RateRuleUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseRateRuleID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		var req rate_ruleRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", 422)
			return
		}
		model := models.NewRateRuleModel(readDB, writeDB, appCache)
		if err := model.Update(id, req.DurationMin, req.AmountCents, req.Label); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				jsonError(w, "not found", 404)
				return
			}
			if isUniqueConstraintErr(err) {
				jsonError(w, "a rate rule for that duration already exists", 409)
				return
			}
			jsonError(w, "failed to update", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// RateRuleDeleteDELETE handles DELETE /api/v1/rate_rules/{id}
func RateRuleDeleteDELETE(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseRateRuleID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewRateRuleModel(readDB, writeDB, appCache)
		if err := model.Delete(id); err != nil {
			jsonError(w, "failed to delete", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

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
	"gova/app/calendar"
	"gova/app/models"
)

type subscriptionRequest struct {
	Name string `json:"name"`
	AmountCents int64 `json:"amount_cents"`
	Cadence string `json:"cadence"`
	BillingDay *int64 `json:"billing_day"`
	IsActive bool `json:"is_active"`
	StartedOn string `json:"started_on"`
	EndedOn *string `json:"ended_on"`
	Notes *string `json:"notes"`
}

func parseSubscriptionID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// SubscriptionListGET handles GET /api/v1/subscriptions
// Query: ?limit=<1..200>&offset=<0..>&sort=<[-]col>&filter=<col:value>
func SubscriptionListGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := queryInt(r, "limit", defaultPageLimit, 1, maxPageLimit)
		offset := queryInt(r, "offset", 0, 0, maxPageOffset)
		opts := models.QueryOpts{Sort: r.URL.Query().Get("sort")}
		if f := r.URL.Query().Get("filter"); f != "" {
			if k, v, ok := strings.Cut(f, ":"); ok {
				opts.FilterField, opts.FilterValue = k, v
			}
		}
		model := models.NewSubscriptionModel(readDB, writeDB, appCache)
		items, total, err := model.GetPage(limit, offset, opts)
		if err != nil {
			if errors.Is(err, models.ErrInvalidQuery) {
				jsonError(w, "invalid sort/filter column; allowed: id, name, amount_cents, cadence, billing_day, is_active, started_on, ended_on, notes, created_at", 422)
				return
			}
			jsonError(w, "failed to load", 500)
			return
		}
		jsonList(w, items, Meta{Limit: limit, Offset: offset, Total: total})
	}
}

// SubscriptionDetailGET handles GET /api/v1/subscriptions/{id}
func SubscriptionDetailGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseSubscriptionID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewSubscriptionModel(readDB, writeDB, appCache)
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

// SubscriptionCreatePOST handles POST /api/v1/subscriptions
func SubscriptionCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req subscriptionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", 422)
			return
		}
		if req.StartedOn == "" {
			// America/New_York wall clock, not the container's UTC clock — see
			// calendar.Now: month membership is a date-range question read
			// straight off started_on/ended_on, so defaulting here to UTC
			// would shift a late-evening ET subscription into tomorrow's
			// month bucket.
			req.StartedOn = calendar.Now().Format("2006-01-02")
		}
		if req.Cadence == "" {
			req.Cadence = "monthly"
		}
		if req.Cadence != "monthly" && req.Cadence != "yearly" && req.Cadence != "weekly" {
			jsonError(w, "cadence must be monthly, yearly, or weekly", 422)
			return
		}
		if !validDate(req.StartedOn) {
			jsonError(w, "started_on must be a valid YYYY-MM-DD date", 422)
			return
		}
		if req.EndedOn != nil && *req.EndedOn != "" && !validDate(*req.EndedOn) {
			jsonError(w, "ended_on must be a valid YYYY-MM-DD date", 422)
			return
		}
		// Month membership is a pure date-range question (see the spec), so
		// is_active must never be a term in it — the same normalization
		// SubscriptionUpdatePUT applies below. Without this, a subscription
		// POSTed with is_active:false and no ended_on would count as live in
		// every month from started_on onward forever, even though the UI
		// shows it struck through.
		if !req.IsActive && (req.EndedOn == nil || *req.EndedOn == "") {
			today := calendar.Now().Format("2006-01-02")
			req.EndedOn = &today
		}
		model := models.NewSubscriptionModel(readDB, writeDB, appCache)
		id, err := model.Create(req.Name, req.AmountCents, req.Cadence, req.BillingDay, req.IsActive, req.StartedOn, req.EndedOn, req.Notes)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// SubscriptionUpdatePUT handles PUT /api/v1/subscriptions/{id}
func SubscriptionUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseSubscriptionID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		var req subscriptionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", 422)
			return
		}
		// Month membership is a pure date-range question (see the spec), so
		// is_active must never be a term in it. Switching a subscription off
		// therefore has to write the end date the range query reads.
		today := calendar.Now().Format("2006-01-02")
		if !req.IsActive && (req.EndedOn == nil || *req.EndedOn == "") {
			req.EndedOn = &today
		}
		if req.IsActive {
			req.EndedOn = nil
		}
		if req.Cadence != "monthly" && req.Cadence != "yearly" && req.Cadence != "weekly" {
			jsonError(w, "cadence must be monthly, yearly, or weekly", 422)
			return
		}
		if !validDate(req.StartedOn) {
			jsonError(w, "started_on must be a valid YYYY-MM-DD date", 422)
			return
		}
		if req.EndedOn != nil && *req.EndedOn != "" && !validDate(*req.EndedOn) {
			jsonError(w, "ended_on must be a valid YYYY-MM-DD date", 422)
			return
		}
		model := models.NewSubscriptionModel(readDB, writeDB, appCache)
		if err := model.Update(id, req.Name, req.AmountCents, req.Cadence, req.BillingDay, req.IsActive, req.StartedOn, req.EndedOn, req.Notes); err != nil {
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

// SubscriptionDeleteDELETE handles DELETE /api/v1/subscriptions/{id}
func SubscriptionDeleteDELETE(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseSubscriptionID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewSubscriptionModel(readDB, writeDB, appCache)
		if err := model.Delete(id); err != nil {
			jsonError(w, "failed to delete", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

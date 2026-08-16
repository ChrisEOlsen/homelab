package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gova/app/cache"
	"gova/app/models"
)

type expenseRequest struct {
	Name string `json:"name"`
	AmountCents int64 `json:"amount_cents"`
	Category *string `json:"category"`
	Status string `json:"status"`
	IncurredOn string `json:"incurred_on"`
	Notes *string `json:"notes"`
}

func parseExpenseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// ExpenseListGET handles GET /api/v1/expenses
// Query: ?limit=<1..200>&offset=<0..>&sort=<[-]col>&filter=<col:value>
func ExpenseListGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := queryInt(r, "limit", defaultPageLimit, 1, maxPageLimit)
		offset := queryInt(r, "offset", 0, 0, maxPageOffset)
		opts := models.QueryOpts{Sort: r.URL.Query().Get("sort")}
		if f := r.URL.Query().Get("filter"); f != "" {
			if k, v, ok := strings.Cut(f, ":"); ok {
				opts.FilterField, opts.FilterValue = k, v
			}
		}
		model := models.NewExpenseModel(readDB, writeDB, appCache)
		items, total, err := model.GetPage(limit, offset, opts)
		if err != nil {
			if errors.Is(err, models.ErrInvalidQuery) {
				jsonError(w, "invalid sort/filter column; allowed: id, name, amount_cents, category, status, incurred_on, notes, created_at", 422)
				return
			}
			jsonError(w, "failed to load", 500)
			return
		}
		jsonList(w, items, Meta{Limit: limit, Offset: offset, Total: total})
	}
}

// ExpenseDetailGET handles GET /api/v1/expenses/{id}
func ExpenseDetailGET(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseExpenseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewExpenseModel(readDB, writeDB, appCache)
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

// ExpenseCreatePOST handles POST /api/v1/expenses
func ExpenseCreatePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req expenseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", 422)
			return
		}
		if req.IncurredOn == "" {
			req.IncurredOn = time.Now().Format("2006-01-02")
		}
		if req.Status == "" {
			req.Status = "planned"
		}
		model := models.NewExpenseModel(readDB, writeDB, appCache)
		id, err := model.Create(req.Name, req.AmountCents, req.Category, req.Status, req.IncurredOn, req.Notes)
		if err != nil {
			jsonError(w, "failed to create", 500)
			return
		}
		jsonOK(w, map[string]int64{"id": id})
	}
}

// ExpenseUpdatePUT handles PUT /api/v1/expenses/{id}
func ExpenseUpdatePUT(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseExpenseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		var req expenseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonError(w, "invalid request body", 422)
			return
		}
		// Marking an item bought moves it into today's month unless the caller
		// supplied a date explicitly — net-per-month should follow when money
		// actually left, not when the item was first wished for.
		if req.Status == "bought" && req.IncurredOn == "" {
			req.IncurredOn = time.Now().Format("2006-01-02")
		}
		if req.IncurredOn == "" {
			jsonError(w, "incurred_on is required", 422)
			return
		}
		model := models.NewExpenseModel(readDB, writeDB, appCache)
		if err := model.Update(id, req.Name, req.AmountCents, req.Category, req.Status, req.IncurredOn, req.Notes); err != nil {
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

// ExpenseDeleteDELETE handles DELETE /api/v1/expenses/{id}
func ExpenseDeleteDELETE(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := parseExpenseID(r)
		if err != nil {
			jsonError(w, "invalid id", 422)
			return
		}
		model := models.NewExpenseModel(readDB, writeDB, appCache)
		if err := model.Delete(id); err != nil {
			jsonError(w, "failed to delete", 500)
			return
		}
		jsonOK(w, map[string]bool{"ok": true})
	}
}

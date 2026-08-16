package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	sqlite3 "github.com/mattn/go-sqlite3"
	"gova/app/models"
)

// parseID reads the {id} path parameter as an int64.
func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

// listQuery reads the standard list-window parameters shared by every resource
// list endpoint: ?limit=<1..200>, ?offset=<0..>, ?sort=<[-]col>, and
// ?filter=<col:value>. Sort/filter columns are validated inside the model
// against its column whitelist.
func listQuery(r *http.Request) (limit, offset int, opts models.QueryOpts) {
	limit = queryInt(r, "limit", defaultPageLimit, 1, maxPageLimit)
	offset = queryInt(r, "offset", 0, 0, maxPageOffset)
	opts.Sort = r.URL.Query().Get("sort")
	if f := r.URL.Query().Get("filter"); f != "" {
		if k, v, ok := strings.Cut(f, ":"); ok {
			opts.FilterField, opts.FilterValue = k, v
		}
	}
	return limit, offset, opts
}

// validDate reports whether value is a well-formed YYYY-MM-DD calendar date.
// It parses rather than length-checks so a string like "2026-13-45" (right
// shape, impossible date) is rejected too — anything stored in a *_on column
// feeds date-string comparisons or slicing in models/, where a bad value
// either mis-buckets money or slice-panics.
func validDate(value string) bool {
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

// isUniqueConstraintErr reports whether err is a SQLite UNIQUE index
// violation (e.g. a duplicate clients.match_name or rate_rules.duration_min).
// Resource handlers backed by a UNIQUE index use this to turn the generic
// write error into a 409/conflict instead of a 500 — a duplicate is a
// routine outcome of the review-queue flow, not an exceptional one. Detected
// via the driver's typed error rather than a message match: mattn/go-sqlite3
// is already an indirect dependency (imported for its side effect in db/),
// so this adds no new one.
func isUniqueConstraintErr(err error) bool {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code == sqlite3.ErrConstraint && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique
	}
	return false
}

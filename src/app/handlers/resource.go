package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
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

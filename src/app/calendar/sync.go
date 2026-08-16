package calendar

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"gova/app/cache"
	"gova/app/models"
)

// DefaultICSURL is the calsync daemon on the home server, reachable from the
// app container over the tailnet interface it binds (HOST in calsync's .env).
const DefaultICSURL = "http://100.85.186.108:5522/calendar.ics"

// runMu serializes whole sync runs process-wide. It is deliberately package
// scope, not a Service field: the generated route signature gives the handler
// only db handles, so it constructs a fresh Service per request, and a
// per-instance mutex would be inert exactly when it matters -- a button press
// landing during a ticker run.
var runMu sync.Mutex

// Result is one sync run, returned to the page and written to calendar_syncs.
//
// events_seen is every event the feed parsed successfully -- not what was
// applied. created + updated is what was actually written to the ledger;
// failed is the difference, the count of events that parsed fine but failed
// to store (e.g. a constraint violation on one row). Do not read events_seen
// as "sessions now in the ledger" -- read created + updated for that.
type Result struct {
	OK         bool   `json:"ok"`
	EventsSeen int    `json:"events_seen"`
	Created    int    `json:"created"`
	Updated    int    `json:"updated"`
	Cancelled  int    `json:"cancelled"`
	Failed     int    `json:"failed"`
	Error      string `json:"error,omitempty"`
	FinishedAt string `json:"finished_at"`
}

// Service owns one sync loop.
type Service struct {
	url      string
	client   *http.Client
	sessions *models.TrainingSessionModel
	clients  *models.ClientModel
	rules    *models.RateRuleModel
	syncs    *models.CalendarSyncModel
}

func NewService(url string, sessions *models.TrainingSessionModel, clients *models.ClientModel,
	rules *models.RateRuleModel, syncs *models.CalendarSyncModel) *Service {
	return &Service{
		url:      url,
		client:   &http.Client{Timeout: 15 * time.Second},
		sessions: sessions,
		clients:  clients,
		rules:    rules,
		syncs:    syncs,
	}
}

// NewFromDB builds a service from the app's handles and CALENDAR_ICS_URL.
func NewFromDB(readDB, writeDB *sql.DB, c *cache.Cache) *Service {
	url := os.Getenv("CALENDAR_ICS_URL")
	if url == "" {
		url = DefaultICSURL
	}
	return NewService(url,
		models.NewTrainingSessionModel(readDB, writeDB, c),
		models.NewClientModel(readDB, writeDB, c),
		models.NewRateRuleModel(readDB, writeDB, c),
		models.NewCalendarSyncModel(readDB, writeDB, c),
	)
}

// Now returns the wall clock in the feed's zone, which is the clock every
// stored stamp is in. Falls back to UTC only if the zone database is missing.
func Now() time.Time {
	loc, err := time.LoadLocation(FeedTimezone)
	if err != nil {
		log.Printf("calendar: failed to load timezone %q, falling back to UTC -- every stored timestamp and the earned/projected boundary will shift by the zone offset until this is fixed: %v", FeedTimezone, err)
		return time.Now().UTC()
	}
	return time.Now().In(loc)
}

// Run performs one whole sync and records it. It never returns an error: an
// unreachable feed is a logged failed run, not a broken endpoint.
func (s *Service) Run(ctx context.Context) Result {
	if !runMu.TryLock() {
		res := Result{FinishedAt: Now().Format("2006-01-02 15:04:05")}
		res.Error = "a sync is already running"
		return res
	}
	defer runMu.Unlock()

	// started stamps last_seen_at on every row this run touches -- it
	// legitimately marks when the run began touching them. res.FinishedAt is
	// a different moment: it is set in record(), right before persisting,
	// so a run that spends time in the HTTP fetch or the reconcile loop
	// reports the wall-clock time it actually finished, not the time it
	// started. Reusing one stamp for both used to make a slow or timed-out
	// run's "Last sync HH:MM:SS" predate the outcome it describes.
	started := Now().Format("2006-01-02 15:04:05")
	res := Result{}

	events, err := s.fetch(ctx)
	if err != nil {
		return s.record(res, err)
	}
	res.EventsSeen = len(events)

	// A zero-event parse is treated as a failure, never as "everything was
	// cancelled". calsync has its own anti-flap guard upstream; this is the
	// second belt, and the one that protects a month of income.
	if len(events) == 0 {
		return s.record(res, fmt.Errorf("feed returned no events — refusing to reconcile"))
	}

	clientRows, err := s.clients.AllForMatching()
	if err != nil {
		return s.record(res, err)
	}
	priceClients := make([]Client, 0, len(clientRows))
	for _, c := range clientRows {
		priceClients = append(priceClients, Client{ID: c.ID, MatchName: c.MatchName, RateCents: c.RateCents, Kind: c.Kind})
	}

	ruleRows, err := s.rules.AllRules()
	if err != nil {
		return s.record(res, err)
	}
	priceRules := make([]RateRule, 0, len(ruleRows))
	for _, r := range ruleRows {
		priceRules = append(priceRules, RateRule{DurationMin: r.DurationMin, AmountCents: r.AmountCents})
	}

	now := started
	seen := make([]string, 0, len(events))
	minDate, maxDate := events[0].Date(), events[0].Date()

	for _, e := range events {
		d := e.Date()
		if d < minDate {
			minDate = d
		}
		if d > maxDate {
			maxDate = d
		}
		seen = append(seen, e.UID)

		p := Resolve(e.Source, e.ClientName, e.DurationMin(), nil, priceClients, priceRules)

		var service *string
		if e.Service != "" {
			svc := e.Service
			service = &svc
		}

		created, err := s.sessions.UpsertFromCalendar(models.TrainingSessionUpsert{
			UID:         e.UID,
			Source:      e.Source,
			ClientName:  e.ClientName,
			ClientID:    p.ClientID,
			Service:     service,
			SessionDate: e.Date(),
			StartAt:     e.StartAt(),
			EndAt:       e.EndAt(),
			DurationMin: e.DurationMin(),
			AmountCents: p.AmountCents,
			RateSource:  p.RateSource,
			Status:      p.Status,
			NeedsReview: p.NeedsReview,
		}, now)
		if err != nil {
			// One bad row must not stall the other ~130 events, and it must
			// not skip reconciliation either -- seen is already appended
			// above, so the seen-set stays complete and CancelMissing below
			// remains safe to run.
			res.Failed++
			continue
		}
		if created {
			res.Created++
		} else {
			res.Updated++
		}
	}

	// The feed contractually covers the current week plus five forward, but
	// nothing enforces that on the dates it actually contains. One event with
	// a wild DTSTART (an upstream year typo, a stale recurring master) would
	// otherwise drag the reconcile floor back years and cancel every ledger
	// row it swept past -- unrecoverable, because the feed can never re-show a
	// past date. Clamp to a window a little wider than the feed's real reach.
	floor := Now().AddDate(0, 0, -14).Format("2006-01-02")
	ceil := Now().AddDate(0, 0, 70).Format("2006-01-02")
	if minDate < floor {
		minDate = floor
	}
	if maxDate > ceil {
		maxDate = ceil
	}

	if minDate > maxDate {
		// Every event fell outside the sane window -- a feed this far off is
		// not one to reconcile against. Events were still upserted above; only
		// the cancel sweep is skipped.
		return s.record(res, fmt.Errorf("all events fell outside the sane reconcile window (%s..%s) -- refusing to reconcile", floor, ceil))
	}

	// Reconcile only inside the window this feed actually covered. Anything
	// older is frozen history the feed can no longer speak about.
	cancelled, err := s.sessions.CancelMissing(minDate, maxDate, seen)
	if err != nil {
		return s.record(res, err)
	}
	res.Cancelled = cancelled

	if res.Failed > 0 {
		res.OK = false
		res.Error = fmt.Sprintf("%d of %d events failed to store", res.Failed, res.EventsSeen)
		return s.record(res, nil)
	}

	res.OK = true
	return s.record(res, nil)
}

func (s *Service) fetch(ctx context.Context) ([]Event, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("calendar feed returned HTTP %d", resp.StatusCode)
	}
	return Parse(resp.Body)
}

func (s *Service) record(res Result, err error) Result {
	// Stamp FinishedAt here, at the moment the run is actually done, not at
	// Run's entry -- see the comment on `started` above.
	res.FinishedAt = Now().Format("2006-01-02 15:04:05")
	if err != nil {
		res.OK = false
		res.Error = err.Error()
	}
	if s.syncs != nil {
		_ = s.syncs.Record(models.CalendarSyncRecord{
			FinishedAt: res.FinishedAt,
			OK:         res.OK,
			EventsSeen: res.EventsSeen,
			Failed:     res.Failed,
			Created:    res.Created,
			Updated:    res.Updated,
			Cancelled:  res.Cancelled,
			Error:      res.Error,
		})
	}
	return res
}

package calendar

import (
	"context"
	"database/sql"
	"fmt"
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

// Result is one sync run, returned to the page and written to calendar_syncs.
type Result struct {
	OK         bool   `json:"ok"`
	EventsSeen int    `json:"events_seen"`
	Created    int    `json:"created"`
	Updated    int    `json:"updated"`
	Cancelled  int    `json:"cancelled"`
	Error      string `json:"error,omitempty"`
	FinishedAt string `json:"finished_at"`
}

// Service owns one sync loop. The mutex keeps the button and the background
// ticker from running the same reconciliation at the same time.
type Service struct {
	url      string
	client   *http.Client
	sessions *models.TrainingSessionModel
	clients  *models.ClientModel
	rules    *models.RateRuleModel
	syncs    *models.CalendarSyncModel
	mu       sync.Mutex
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
		return time.Now().UTC()
	}
	return time.Now().In(loc)
}

// Run performs one whole sync and records it. It never returns an error: an
// unreachable feed is a logged failed run, not a broken endpoint.
func (s *Service) Run(ctx context.Context) Result {
	s.mu.Lock()
	defer s.mu.Unlock()

	res := Result{FinishedAt: Now().Format("2006-01-02 15:04:05")}

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

	now := res.FinishedAt
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
			return s.record(res, err)
		}
		if created {
			res.Created++
		} else {
			res.Updated++
		}
	}

	// Reconcile only inside the window this feed actually covered. Anything
	// older is frozen history the feed can no longer speak about.
	cancelled, err := s.sessions.CancelMissing(minDate, maxDate, seen)
	if err != nil {
		return s.record(res, err)
	}
	res.Cancelled = cancelled
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
	if err != nil {
		res.OK = false
		res.Error = err.Error()
	}
	if s.syncs != nil {
		_ = s.syncs.Record(models.CalendarSyncRecord{
			FinishedAt: res.FinishedAt,
			OK:         res.OK,
			EventsSeen: res.EventsSeen,
			Created:    res.Created,
			Updated:    res.Updated,
			Cancelled:  res.Cancelled,
			Error:      res.Error,
		})
	}
	return res
}

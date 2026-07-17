package push

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"gova/app/cache"
	"gova/app/models"
)

// httpClient is used for all push sends so a hung connection to the push
// relay can't block the rest of a tick's sends indefinitely.
var httpClient = &http.Client{Timeout: 10 * time.Second}

// maxTransientRetries bounds how many consecutive ticks a reminder may be
// retried after a transient send failure before the scheduler gives up on
// it. At the 1-minute tick interval this is a 5-minute retry window — long
// enough for a real network blip to resolve, short enough that a reminder
// with a persistently-broken subscription (e.g. corrupted stored keys that
// fail at the Go level on every attempt) doesn't retry forever and re-spam
// every other subscription on the same reminder indefinitely.
const maxTransientRetries = 5

// transientFailureCounts tracks consecutive transient-failure attempts per
// reminder ID, in-process. It's read/written only from the single scheduler
// ticker goroutine, but guarded by a mutex for safety/clarity. State is lost
// on restart, which is fine: a restart effectively resets the retry window,
// consistent with this app's existing in-process cache convention.
var (
	transientFailureMu     sync.Mutex
	transientFailureCounts = make(map[int64]int)
)

type payload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// Start launches the background reminder-check loop in its own goroutine
// and returns immediately. Called once from main.go.
func Start(readDB, writeDB *sql.DB, appCache *cache.Cache, vapidPublicKey, vapidPrivateKey, subscriber string) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("push: recovered from panic in checkAndSend: %v", r)
					}
				}()
				checkAndSend(readDB, writeDB, appCache, vapidPublicKey, vapidPrivateKey, subscriber)
			}()
		}
	}()
}

func checkAndSend(readDB, writeDB *sql.DB, appCache *cache.Cache, vapidPublicKey, vapidPrivateKey, subscriber string) {
	reminderModel := models.NewReminderModel(readDB, writeDB, appCache)
	due, err := reminderModel.GetDueUnnotified()
	if err != nil {
		log.Printf("push: failed to query due reminders: %v", err)
		return
	}
	if len(due) == 0 {
		return
	}

	subModel := models.NewPushSubscriptionModel(readDB, writeDB)
	subs, err := subModel.GetAll()
	if err != nil {
		log.Printf("push: failed to load subscriptions: %v", err)
		return
	}

	for _, reminder := range due {
		body, err := json.Marshal(payload{Title: "Reminder", Body: reminder.Title})
		if err != nil {
			log.Printf("push: failed to marshal payload for reminder %d: %v", reminder.ID, err)
			continue
		}
		hadTransientFailure := false
		for _, sub := range subs {
			webpushSub := &webpush.Subscription{
				Endpoint: sub.Endpoint,
				Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
			}
			resp, err := webpush.SendNotification(body, webpushSub, &webpush.Options{
				HTTPClient:      httpClient,
				Subscriber:      subscriber,
				VAPIDPublicKey:  vapidPublicKey,
				VAPIDPrivateKey: vapidPrivateKey,
				TTL:             60,
			})
			if err != nil {
				// Network/transport-level failure (e.g. timeout, DNS, connection
				// refused). We don't know if the relay ever saw this — treat it
				// as transient so the reminder gets retried next tick instead of
				// silently losing the user's one notification.
				log.Printf("push: send failed for reminder %d: %v", reminder.ID, err)
				hadTransientFailure = true
				continue
			}
			resp.Body.Close()
			switch {
			case resp.StatusCode == 404 || resp.StatusCode == 410:
				// Subscription is permanently gone — nothing to retry for it.
				if err := subModel.DeleteByEndpoint(sub.Endpoint); err != nil {
					log.Printf("push: failed to remove stale subscription: %v", err)
				}
			case resp.StatusCode >= 500:
				// Push relay had a server-side problem — transient, retry next tick.
				log.Printf("push: send failed for reminder %d: relay returned %d", reminder.ID, resp.StatusCode)
				hadTransientFailure = true
			case resp.StatusCode >= 400:
				// Other permanent 4xx (e.g. bad request) — not retryable, but not
				// this reminder's fault to keep retrying forever either.
				log.Printf("push: send rejected for reminder %d: relay returned %d", reminder.ID, resp.StatusCode)
			}
		}
		// Only mark notified once nothing about this reminder is worth retrying:
		// zero subscriptions, or every subscription outcome was a success or a
		// permanent failure (404/410 already deleted above, or other 4xx). If any
		// subscription failed transiently (network error or 5xx), leave
		// notified_at NULL so the next tick retries this reminder — unless it has
		// now hit the retry cap, in which case we give up rather than risk
		// retrying forever and re-sending to already-succeeded subscriptions on
		// every future tick.
		if hadTransientFailure {
			transientFailureMu.Lock()
			transientFailureCounts[reminder.ID]++
			attempts := transientFailureCounts[reminder.ID]
			giveUp := attempts >= maxTransientRetries
			if giveUp {
				delete(transientFailureCounts, reminder.ID)
			}
			transientFailureMu.Unlock()

			if !giveUp {
				continue
			}
			log.Printf("push: giving up on reminder %d after %d consecutive transient failures", reminder.ID, attempts)
		} else {
			transientFailureMu.Lock()
			delete(transientFailureCounts, reminder.ID)
			transientFailureMu.Unlock()
		}
		if err := reminderModel.MarkNotified(reminder.ID); err != nil {
			log.Printf("push: failed to mark reminder %d notified: %v", reminder.ID, err)
		}
	}
}

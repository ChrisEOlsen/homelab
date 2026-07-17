package push

import (
	"database/sql"
	"encoding/json"
	"log"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"gova/app/cache"
	"gova/app/models"
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
			checkAndSend(readDB, writeDB, appCache, vapidPublicKey, vapidPrivateKey, subscriber)
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
		for _, sub := range subs {
			webpushSub := &webpush.Subscription{
				Endpoint: sub.Endpoint,
				Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
			}
			resp, err := webpush.SendNotification(body, webpushSub, &webpush.Options{
				Subscriber:      subscriber,
				VAPIDPublicKey:  vapidPublicKey,
				VAPIDPrivateKey: vapidPrivateKey,
				TTL:             60,
			})
			if err != nil {
				log.Printf("push: send failed for reminder %d: %v", reminder.ID, err)
				continue
			}
			resp.Body.Close()
			if resp.StatusCode == 404 || resp.StatusCode == 410 {
				if err := subModel.DeleteByEndpoint(sub.Endpoint); err != nil {
					log.Printf("push: failed to remove stale subscription: %v", err)
				}
			}
		}
		// Marked regardless of per-subscription outcome — a bad subscription
		// must not block this reminder from ever being marked handled.
		if err := reminderModel.MarkNotified(reminder.ID); err != nil {
			log.Printf("push: failed to mark reminder %d notified: %v", reminder.ID, err)
		}
	}
}

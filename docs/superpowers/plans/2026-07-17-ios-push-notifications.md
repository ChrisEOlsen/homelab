# iOS Push Notifications Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Push a real iOS notification to the user's home-screen-installed shortcut when a reminder's `remind_at` passes, using standard Web Push (no third-party service, no Discord/ntfy).

**Architecture:** An in-process `time.Ticker` goroutine (started once from `main.go`, package `gova/app/push`) polls SQLite every minute for reminders that are active, past due, and not yet notified, then sends an encrypted Web Push message via `github.com/SherClockHolmes/webpush-go` to every stored browser subscription. The frontend adds a PWA manifest + service worker (`static/sw.js`) so the existing home-screen shortcut becomes push-capable, plus one "Enable Notifications" button on the dashboard that registers a subscription.

**Tech Stack:** Go (chi router, existing `models`/`handlers`/`cache` packages), `github.com/SherClockHolmes/webpush-go` v1.4.0, vanilla ES modules, SQLite.

**Full design reference:** `docs/superpowers/specs/2026-07-17-ios-push-notifications-design.md`

## Global Constraints

- **No raw SQL in handlers** — model methods only (`CLAUDE.md`).
- **No HTML rendering in Go handlers** — JSON only, via `jsonOK`/`jsonError` (`handlers/json.go`).
- **JS safety** — `textContent`/`createElement`, never `innerHTML` with dynamic data; all fetches go through `static/js/lib/api.js` (`get`/`post`/`put`/`del`), never raw `fetch()`.
- **`id INTEGER PRIMARY KEY`** for any new table, no `AUTOINCREMENT` (`CLAUDE.md` Golden Recipe).
- **Database first** — new tables/columns created via the `execute_sql` MCP tool before any model code.
- **No Node/NPM** — no new build tooling; icons are generated once at plan-authoring time with host tools (`rsvg-convert`), not as part of the app's runtime build.
- **No existing Go test files anywhere in this project** (`find src/app -name '*_test.go'` → 0 results). This codebase is verified via `curl` against the running dev container and Playwright browser scripts, never `go test`. Follow that established convention — do not introduce a `testing`-package suite as a parallel, unprecedented verification style.
- **One-shot notification only** — no recurrence-advancing engine (matches existing reminders behavior, which never auto-advances `remind_at` either). See spec's "Out of scope."
- **Dev server**: `docker compose up -d app` (from repo root); static JS/HTML edits are live (`http.FileServer`, no restart needed); Go/handler edits need `docker compose restart app` (rebuilds the binary, per `entrypoint.sh`).

---

### Task 1: Database migration (local dev)

**Files:** none (MCP tool call only — `data/app.db`)

- [ ] **Step 1: Back up the local dev database**

```bash
cp /Users/crispychris/Desktop/repos/homelab/data/app.db /Users/crispychris/Desktop/repos/homelab/data/app.db.bak-pre-push-migration
```

- [ ] **Step 2: Call the `execute_sql` MCP tool with this exact SQL**

```sql
ALTER TABLE reminders ADD COLUMN notified_at DATETIME;

CREATE TABLE push_subscriptions (
    id INTEGER PRIMARY KEY,
    endpoint TEXT NOT NULL UNIQUE,
    p256dh TEXT NOT NULL,
    auth TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

- [ ] **Step 3: Verify the migration**

```bash
python3 -c "
import sqlite3
con = sqlite3.connect('/Users/crispychris/Desktop/repos/homelab/data/app.db')
cur = con.cursor()
cur.execute('PRAGMA table_info(reminders)')
print('reminders:', cur.fetchall())
cur.execute('PRAGMA table_info(push_subscriptions)')
print('push_subscriptions:', cur.fetchall())
"
```

Expected: `reminders` now includes a `notified_at` column (type `DATETIME`, nullable); `push_subscriptions` shows all 5 columns above.

---

### Task 2: Add `webpush-go` dependency

**Files:**
- Modify: `src/app/go.mod`, `src/app/go.sum`

**Interfaces:**
- Produces: `github.com/SherClockHolmes/webpush-go` importable as `webpush "github.com/SherClockHolmes/webpush-go"`, exposing:
  - `webpush.Keys{Auth, P256dh string}`
  - `webpush.Subscription{Endpoint string, Keys Keys}`
  - `webpush.Options{Subscriber, VAPIDPublicKey, VAPIDPrivateKey string; TTL int}`
  - `func webpush.SendNotification(payload []byte, s *Subscription, opts *Options) (*http.Response, error)`
  - `func webpush.GenerateVAPIDKeys() (privateKey, publicKey string, err error)`

- [ ] **Step 1: Add the dependency**

```bash
cd /Users/crispychris/Desktop/repos/homelab/src/app && go get github.com/SherClockHolmes/webpush-go@v1.4.0
```

- [ ] **Step 2: Verify it resolves and the module still builds**

```bash
cd /Users/crispychris/Desktop/repos/homelab/src/app && go build ./... && grep webpush-go go.mod
```

Expected: build succeeds with no errors; `go.mod` shows `github.com/SherClockHolmes/webpush-go v1.4.0` in `require`.

- [ ] **Step 3: Commit**

```bash
cd /Users/crispychris/Desktop/repos/homelab && git add src/app/go.mod src/app/go.sum && git commit -m "chore: add webpush-go dependency for push notifications"
```

---

### Task 3: Generate VAPID keys and configure environment

**Files:**
- Modify: `.env` (repo root — gitignored, not committed; already flows to prod via `sync.sh`)

**Interfaces:**
- Produces: env vars `VAPID_PUBLIC_KEY`, `VAPID_PRIVATE_KEY`, `VAPID_SUBSCRIBER`, `TZ` that later tasks read via `os.Getenv`.

- [ ] **Step 1: Generate a VAPID keypair**

```bash
cd /Users/crispychris/Desktop/repos/homelab/src/app && cat > /tmp/genvapid.go << 'EOF'
package main

import (
	"fmt"
	webpush "github.com/SherClockHolmes/webpush-go"
)

func main() {
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		panic(err)
	}
	fmt.Println("VAPID_PUBLIC_KEY=" + pub)
	fmt.Println("VAPID_PRIVATE_KEY=" + priv)
}
EOF
go run /tmp/genvapid.go
rm /tmp/genvapid.go
```

Expected: prints two lines, `VAPID_PUBLIC_KEY=...` and `VAPID_PRIVATE_KEY=...` (base64url strings).

- [ ] **Step 2: Append the generated values plus subscriber contact and timezone to `.env`**

Add these lines to `/Users/crispychris/Desktop/repos/homelab/.env` (paste the actual values printed in Step 1 — do not commit this file, it's already gitignored):

```
# Web Push (iOS notifications for reminders)
VAPID_PUBLIC_KEY=<paste from Step 1>
VAPID_PRIVATE_KEY=<paste from Step 1>
VAPID_SUBSCRIBER=mailto:admin@localhost
TZ=America/New_York
```

- [ ] **Step 3: Recreate the container and verify the env vars are visible inside it**

`docker compose restart` reuses the existing container's already-baked environment and does **not** re-read `.env` — only `docker compose up -d` (which recreates the container from the current compose config) picks up new or changed env vars. This distinction matters only for `.env` changes; later tasks that change only source code can use plain `restart`, since bind-mounted code is picked up either way and the environment doesn't need to change again.

```bash
cd /Users/crispychris/Desktop/repos/homelab && docker compose up -d app
docker compose exec app sh -c 'echo $VAPID_PUBLIC_KEY | cut -c1-10; echo $TZ; date'
```

Expected: first line prints the first 10 chars of the public key (non-empty); `TZ` prints `America/New_York`; `date` shows Eastern time, not UTC.

---

### Task 4: `PushSubscription` model

**Files:**
- Create: `src/app/models/PushSubscription.go`

**Interfaces:**
- Consumes: `*sql.DB` (readDB, writeDB) — same pattern as every other model in this codebase (e.g. `models.NewReminderModel`)
- Produces: `models.NewPushSubscriptionModel(readDB, writeDB *sql.DB) *PushSubscriptionModel` with methods `Create(endpoint, p256dh, auth string) error`, `GetAll() ([]PushSubscription, error)`, `DeleteByEndpoint(endpoint string) error`

- [ ] **Step 1: Write the model**

```go
package models

import (
	"database/sql"
)

type PushSubscription struct {
	ID        int64  `json:"id"`
	Endpoint  string `json:"endpoint"`
	P256dh    string `json:"p256dh"`
	Auth      string `json:"auth"`
	CreatedAt string `json:"created_at"`
}

type PushSubscriptionModel struct {
	readDB  *sql.DB
	writeDB *sql.DB
}

func NewPushSubscriptionModel(readDB, writeDB *sql.DB) *PushSubscriptionModel {
	return &PushSubscriptionModel{readDB: readDB, writeDB: writeDB}
}

// Create is idempotent per endpoint — re-subscribing the same device
// (e.g. after reopening the app) must not create a duplicate row.
func (m *PushSubscriptionModel) Create(endpoint, p256dh, auth string) error {
	_, err := m.writeDB.Exec(
		"INSERT OR IGNORE INTO push_subscriptions (endpoint, p256dh, auth) VALUES (?, ?, ?)",
		endpoint, p256dh, auth,
	)
	return err
}

func (m *PushSubscriptionModel) GetAll() ([]PushSubscription, error) {
	rows, err := m.readDB.Query("SELECT id, endpoint, p256dh, auth, created_at FROM push_subscriptions")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PushSubscription
	for rows.Next() {
		var item PushSubscription
		if err := rows.Scan(&item.ID, &item.Endpoint, &item.P256dh, &item.Auth, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// DeleteByEndpoint is called when webpush-go's SendNotification reports the
// subscription is gone (404/410) — the browser install was uninstalled or
// the subscription expired, so it self-cleans instead of erroring forever.
func (m *PushSubscriptionModel) DeleteByEndpoint(endpoint string) error {
	_, err := m.writeDB.Exec("DELETE FROM push_subscriptions WHERE endpoint = ?", endpoint)
	return err
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /Users/crispychris/Desktop/repos/homelab/src/app && go build ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
cd /Users/crispychris/Desktop/repos/homelab && git add src/app/models/PushSubscription.go && git commit -m "feat: add PushSubscription model"
```

---

### Task 5: `Reminder` model additions

**Files:**
- Modify: `src/app/models/Reminder.go`

**Interfaces:**
- Consumes: existing `ReminderModel{readDB, writeDB, cache}` (already defined in this file)
- Produces: `(m *ReminderModel) GetDueUnnotified() ([]Reminder, error)`, `(m *ReminderModel) MarkNotified(id int64) error`

- [ ] **Step 1: Add `"time"` handling and the two new methods**

Add these two methods to the end of `src/app/models/Reminder.go` (the file already imports `"time"` for `CreatedAt time.Time`, so no new import is needed):

```go
// GetDueUnnotified returns active reminders whose remind_at has passed and
// that haven't been pushed yet. One-shot only — matches the fact that
// recurrence_type/recurrence_days are stored but never auto-advance
// remind_at anywhere else in this app either.
//
// remind_at is stored as a naive "YYYY-MM-DDTHH:MM" string with no timezone
// (straight from the browser's <input type="datetime-local">). Comparison
// is done in Go using time.Local (set via the TZ env var — see .env) rather
// than in SQL, since a raw string comparison against a Go-formatted "now"
// is fragile whenever seconds are present on one side and not the other.
func (m *ReminderModel) GetDueUnnotified() ([]Reminder, error) {
	rows, err := m.readDB.Query(
		"SELECT id, title, remind_at, recurrence_type, recurrence_days, is_active, created_at FROM reminders WHERE is_active = 1 AND notified_at IS NULL",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Reminder
	for rows.Next() {
		var item Reminder
		var recurrenceDays sql.NullString
		if err := rows.Scan(&item.ID, &item.Title, &item.RemindAt, &item.RecurrenceType, &recurrenceDays, &item.IsActive, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.RecurrenceDays = recurrenceDays.String
		due, err := time.ParseInLocation("2006-01-02T15:04", item.RemindAt, time.Local)
		if err != nil || due.After(time.Now()) {
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

func (m *ReminderModel) MarkNotified(id int64) error {
	_, err := m.writeDB.Exec("UPDATE reminders SET notified_at = CURRENT_TIMESTAMP WHERE id = ?", id)
	if err == nil {
		m.cache.Bust("reminders:")
	}
	return err
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /Users/crispychris/Desktop/repos/homelab/src/app && go build ./...
```

Expected: no errors.

- [ ] **Step 3: Manually verify the query logic against real data**

```bash
cd /Users/crispychris/Desktop/repos/homelab && docker compose restart app
curl -s -X POST http://localhost:1234/api/reminders_create \
  -H 'Content-Type: application/json' \
  -d '{"title":"push test reminder","remind_at":"2020-01-01T00:00","recurrence_type":"none","recurrence_days":""}'
python3 -c "
import sqlite3
con = sqlite3.connect('/Users/crispychris/Desktop/repos/homelab/data/app.db')
cur = con.cursor()
cur.execute(\"SELECT id, title, remind_at, notified_at FROM reminders WHERE title = 'push test reminder'\")
print(cur.fetchall())
"
```

Expected: one row with a past `remind_at` (`2020-01-01T00:00`) and `notified_at` still `None` (not yet touched by anything — this reminder will be picked up once the scheduler exists in Task 7). Leave this row in place; Task 8's manual test reuses it.

- [ ] **Step 4: Commit**

```bash
cd /Users/crispychris/Desktop/repos/homelab && git add src/app/models/Reminder.go && git commit -m "feat: add GetDueUnnotified/MarkNotified to Reminder model"
```

---

### Task 6: Subscribe + public-key handlers

**Files:**
- Create: `src/app/handlers/push_subscribe.go`
- Create: `src/app/handlers/push_public_key.go`
- Modify: `src/app/main.go` (add two routes)

**Interfaces:**
- Consumes: `models.NewPushSubscriptionModel` (Task 4)
- Produces: `POST /api/push_subscribe` (body `{endpoint, keys: {p256dh, auth}}` → `{ok:true}`), `GET /api/push_public_key` (→ `{ok:true, data:{key:"..."}}`)

- [ ] **Step 1: Write the subscribe handler**

```go
package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"gova/app/cache"
	"gova/app/models"
)

func PushSubscribePOST(readDB, writeDB *sql.DB, appCache *cache.Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Endpoint string `json:"endpoint"`
			Keys     struct {
				P256dh string `json:"p256dh"`
				Auth   string `json:"auth"`
			} `json:"keys"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil ||
			body.Endpoint == "" || body.Keys.P256dh == "" || body.Keys.Auth == "" {
			jsonError(w, "endpoint and keys are required", 400)
			return
		}
		model := models.NewPushSubscriptionModel(readDB, writeDB)
		if err := model.Create(body.Endpoint, body.Keys.P256dh, body.Keys.Auth); err != nil {
			jsonError(w, "failed to save subscription", 500)
			return
		}
		jsonOK(w, nil)
	}
}
```

- [ ] **Step 2: Write the public-key handler**

```go
package handlers

import (
	"net/http"
	"os"
)

func PushPublicKeyGET() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := os.Getenv("VAPID_PUBLIC_KEY")
		if key == "" {
			jsonError(w, "push notifications are not configured", 503)
			return
		}
		jsonOK(w, map[string]string{"key": key})
	}
}
```

- [ ] **Step 3: Register both routes in `main.go`**

Add these two lines directly below the existing reminders routes (after line 54, `r.Post("/api/reminders/{id}/toggle", ...)`):

```go
	r.Post("/api/push_subscribe", handlers.PushSubscribePOST(database.Read, database.Write, appCache))
	r.Get("/api/push_public_key", handlers.PushPublicKeyGET())
```

- [ ] **Step 4: Restart and verify both endpoints**

```bash
cd /Users/crispychris/Desktop/repos/homelab && docker compose restart app
curl -s http://localhost:1234/api/push_public_key
curl -s -X POST http://localhost:1234/api/push_subscribe -H 'Content-Type: application/json' -d '{"endpoint":"https://example.com/ep1","keys":{"p256dh":"abc","auth":"def"}}'
curl -s -X POST http://localhost:1234/api/push_subscribe -H 'Content-Type: application/json' -d '{"endpoint":"https://example.com/ep1","keys":{"p256dh":"abc","auth":"def"}}'
python3 -c "
import sqlite3
con = sqlite3.connect('/Users/crispychris/Desktop/repos/homelab/data/app.db')
print(con.execute('SELECT endpoint FROM push_subscriptions').fetchall())
"
```

Expected: `/api/push_public_key` returns `{"ok":true,"data":{"key":"..."}}`; both subscribe calls return `{"ok":true}`; the DB query shows exactly **one** row for `https://example.com/ep1` (proving `INSERT OR IGNORE` deduplicates). Delete this test row before moving on:

```bash
python3 -c "
import sqlite3
con = sqlite3.connect('/Users/crispychris/Desktop/repos/homelab/data/app.db')
con.execute(\"DELETE FROM push_subscriptions WHERE endpoint = 'https://example.com/ep1'\")
con.commit()
"
```

- [ ] **Step 5: Commit**

```bash
cd /Users/crispychris/Desktop/repos/homelab && git add src/app/handlers/push_subscribe.go src/app/handlers/push_public_key.go src/app/main.go && git commit -m "feat: add push subscribe and public-key endpoints"
```

---

### Task 7: Background scheduler

**Files:**
- Create: `src/app/push/scheduler.go`
- Modify: `src/app/main.go` (start the scheduler)

**Interfaces:**
- Consumes: `models.NewReminderModel(...).GetDueUnnotified()`/`.MarkNotified()` (Task 5), `models.NewPushSubscriptionModel(...).GetAll()`/`.DeleteByEndpoint()` (Task 4), `webpush.SendNotification` (Task 2)
- Produces: `push.Start(readDB, writeDB *sql.DB, appCache *cache.Cache, vapidPublicKey, vapidPrivateKey, subscriber string)` — starts its own goroutine, returns immediately

- [ ] **Step 1: Write the scheduler**

```go
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
```

- [ ] **Step 2: Wire it into `main.go`**

Add `"gova/app/push"` to the import block, and add this right after the `appCache := cache.New()` / `_ = appCache` lines (before `r := chi.NewRouter()`):

```go
	if vapidPublicKey, vapidPrivateKey := os.Getenv("VAPID_PUBLIC_KEY"), os.Getenv("VAPID_PRIVATE_KEY"); vapidPublicKey == "" || vapidPrivateKey == "" {
		log.Println("VAPID_PUBLIC_KEY/VAPID_PRIVATE_KEY not set — push notifications disabled")
	} else {
		subscriber := os.Getenv("VAPID_SUBSCRIBER")
		if subscriber == "" {
			subscriber = "mailto:admin@localhost"
		}
		push.Start(database.Read, database.Write, appCache, vapidPublicKey, vapidPrivateKey, subscriber)
	}
```

- [ ] **Step 3: Verify it compiles and starts**

```bash
cd /Users/crispychris/Desktop/repos/homelab && docker compose restart app
sleep 2
docker compose logs app --tail 20
```

Expected: no "VAPID_PUBLIC_KEY/VAPID_PRIVATE_KEY not set" warning (Task 3 already configured `.env`); no panics.

- [ ] **Step 4: End-to-end verify against a mock push endpoint**

This proves the scheduler correctly finds due reminders and calls `SendNotification` — without needing a real phone. Run a tiny mock push server, subscribe to it, wait for a tick, and check `notified_at`:

```bash
python3 << 'EOF'
import http.server, threading, sqlite3, time as t, json, urllib.request

received = []

class Handler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        received.append(self.path)
        self.send_response(201)
        self.end_headers()
    def log_message(self, *args):
        pass

server = http.server.HTTPServer(('0.0.0.0', 9999), Handler)
threading.Thread(target=server.serve_forever, daemon=True).start()

# Subscribe using the mock endpoint (reachable from inside the app
# container via host.docker.internal on Docker Desktop for Mac).
sub = {
    "endpoint": "http://host.docker.internal:9999/mock-endpoint",
    "keys": {
        # A real P-256 keypair's public point + random 16-byte auth secret —
        # NOT a real browser subscription, but a syntactically and
        # cryptographically valid one, so webpush-go's ECDH step actually
        # succeeds and the HTTP POST really reaches the mock server below
        # (fabricated/random-looking bytes fail ECDH before any network call
        # is attempted, since they don't lie on the curve).
        "p256dh": "BKTsnmwps8Vzysl4poBzolwJRTlITJkvRNMzAdn_0obmjczVgqZDhUmhI1jjQwoUZc2-R_WnCwM5N9OVm_dQQUQ",
        "auth": "byNvFb5Qmfy4s0AJ--WrgA"
    }
}
req = urllib.request.Request(
    "http://localhost:1234/api/push_subscribe",
    data=json.dumps(sub).encode(),
    headers={"Content-Type": "application/json"},
    method="POST",
)
print("subscribe:", urllib.request.urlopen(req).read())

con = sqlite3.connect('/Users/crispychris/Desktop/repos/homelab/data/app.db')
before = con.execute("SELECT notified_at FROM reminders WHERE title = 'push test reminder'").fetchone()
print("notified_at before:", before)

print("waiting up to 70s for the ticker to fire...")
t.sleep(70)

con2 = sqlite3.connect('/Users/crispychris/Desktop/repos/homelab/data/app.db')
after = con2.execute("SELECT notified_at FROM reminders WHERE title = 'push test reminder'").fetchone()
print("notified_at after:", after)
print("mock server received requests:", received)
EOF
```

Expected: `notified_at before` is `(None,)`; `notified_at after` is a non-null timestamp; `received` contains one path matching `/mock-endpoint` (proving `SendNotification` completed real VAPID signing + RFC 8291 encryption against a genuine P-256 point and the resulting HTTP request actually reached the subscription's endpoint). The mock server doesn't implement the real Web Push relay protocol, so it can't validate the encrypted payload's contents — this step proves the send path executes correctly end-to-end at the transport level, not that Apple's relay will accept the payload. Real end-to-end delivery is confirmed against a genuine browser subscription in Task 11.

- [ ] **Step 5: Clean up test data**

```bash
python3 -c "
import sqlite3
con = sqlite3.connect('/Users/crispychris/Desktop/repos/homelab/data/app.db')
con.execute(\"DELETE FROM reminders WHERE title = 'push test reminder'\")
con.execute(\"DELETE FROM push_subscriptions WHERE endpoint LIKE '%mock-endpoint%'\")
con.commit()
"
```

- [ ] **Step 6: Commit**

```bash
cd /Users/crispychris/Desktop/repos/homelab && git add src/app/push/scheduler.go src/app/main.go && git commit -m "feat: add background scheduler to send due reminder push notifications"
```

---

### Task 8: App icon

**Files:**
- Create: `src/app/static/icons/icon.svg`
- Create: `src/app/static/icons/icon-180.png`
- Create: `src/app/static/icons/icon-192.png`
- Create: `src/app/static/icons/icon-512.png`

- [ ] **Step 1: Write the source SVG**

Matches the existing brand mark already in every page header (`<span class="h-2 w-2 rounded-full bg-accent led-pulse">` next to "HOMELAB") — the accent dot, at icon scale, on the app's `--color-ink` background:

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
  <rect width="512" height="512" fill="#2b2620"/>
  <circle cx="256" cy="256" r="120" fill="#a8632b"/>
</svg>
```

Save this to `src/app/static/icons/icon.svg`.

- [ ] **Step 2: Rasterize to the three required sizes**

```bash
cd /Users/crispychris/Desktop/repos/homelab/src/app/static/icons
rsvg-convert -w 180 -h 180 icon.svg -o icon-180.png
rsvg-convert -w 192 -h 192 icon.svg -o icon-192.png
rsvg-convert -w 512 -h 512 icon.svg -o icon-512.png
```

- [ ] **Step 3: Verify the files exist and are valid PNGs**

```bash
cd /Users/crispychris/Desktop/repos/homelab/src/app/static/icons && file icon-180.png icon-192.png icon-512.png
```

Expected: each line reports `PNG image data` with the matching dimensions (`180 x 180`, `192 x 192`, `512 x 512`).

- [ ] **Step 4: Commit**

```bash
cd /Users/crispychris/Desktop/repos/homelab && git add src/app/static/icons/ && git commit -m "feat: add app icon for PWA manifest and notifications"
```

---

### Task 9: Manifest + service worker

**Files:**
- Create: `src/app/static/manifest.json`
- Create: `src/app/static/sw.js`
- Modify: `src/app/main.go` (serve `sw.js` from the document root)

**Interfaces:**
- Produces: `GET /manifest.json` → static/manifest.json content (served via existing `/static/*` handler, referenced as `/static/manifest.json`); `GET /sw.js` → the service worker script, served from the root path so its default scope covers the whole site

- [ ] **Step 1: Write the manifest**

```json
{
  "name": "Homelab",
  "short_name": "Homelab",
  "start_url": "/",
  "display": "standalone",
  "background_color": "#faf7f2",
  "theme_color": "#2b2620",
  "icons": [
    { "src": "/static/icons/icon-192.png", "sizes": "192x192", "type": "image/png" },
    { "src": "/static/icons/icon-512.png", "sizes": "512x512", "type": "image/png" }
  ]
}
```

Save to `src/app/static/manifest.json`.

- [ ] **Step 2: Write the service worker**

```js
self.addEventListener('push', (event) => {
  const data = event.data ? event.data.json() : {};
  const title = data.title || 'Homelab';
  const options = {
    body: data.body || '',
    icon: '/static/icons/icon-192.png',
  };
  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clientList) => {
      for (const client of clientList) {
        if ('focus' in client) return client.focus();
      }
      if (clients.openWindow) return clients.openWindow('/');
    })
  );
});
```

Save to `src/app/static/sw.js`.

- [ ] **Step 3: Serve `sw.js` from the document root in `main.go`**

A service worker's default scope is the directory it's served from — registering it at `/static/sw.js` would only let it control pages under `/static/`, not the whole site. Add this route right below the existing `r.Handle("/static/*", ...)` line:

```go
	r.Get("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/sw.js")
	})
```

- [ ] **Step 4: Verify both are reachable**

```bash
cd /Users/crispychris/Desktop/repos/homelab && docker compose restart app
curl -s http://localhost:1234/sw.js | head -3
curl -s http://localhost:1234/static/manifest.json
```

Expected: `sw.js` prints the `self.addEventListener('push', ...` line; `manifest.json` prints the JSON from Step 1.

- [ ] **Step 5: Commit**

```bash
cd /Users/crispychris/Desktop/repos/homelab && git add src/app/static/manifest.json src/app/static/sw.js src/app/main.go && git commit -m "feat: add PWA manifest and service worker"
```

---

### Task 10: Shared `push.js` frontend helper

**Files:**
- Create: `src/app/static/js/lib/push.js`

**Interfaces:**
- Consumes: `get`/`post` from `/static/js/lib/api.js` (existing), `GET /api/push_public_key` (Task 6), `POST /api/push_subscribe` (Task 6), `/sw.js` (Task 9)
- Produces: `export async function registerServiceWorker()`, `export async function enablePushNotifications()` — return `{ ok: true }` or `{ ok: false, error: string }`

- [ ] **Step 1: Write the helper**

```js
import { get, post } from '/static/js/lib/api.js';

function urlBase64ToUint8Array(base64String) {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const rawData = atob(base64);
  const outputArray = new Uint8Array(rawData.length);
  for (let i = 0; i < rawData.length; i++) {
    outputArray[i] = rawData.charCodeAt(i);
  }
  return outputArray;
}

// Registers the service worker on every page load. Safe to call
// unconditionally and repeatedly — registration is idempotent, and this
// alone requests no permission and creates no subscription.
export async function registerServiceWorker() {
  if (!('serviceWorker' in navigator)) return null;
  return navigator.serviceWorker.register('/sw.js');
}

// Requests notification permission and creates a push subscription. Must be
// called from a user gesture (e.g. a button click) — iOS Safari requires
// this and running inside the installed home-screen app, not a regular tab.
export async function enablePushNotifications() {
  if (!('serviceWorker' in navigator) || !('PushManager' in window)) {
    return { ok: false, error: 'Push notifications are not supported in this browser.' };
  }

  const permission = await Notification.requestPermission();
  if (permission !== 'granted') {
    return { ok: false, error: 'Notification permission was not granted.' };
  }

  const registration = await navigator.serviceWorker.ready;

  const keyRes = await get('/api/push_public_key');
  if (!keyRes.ok || !keyRes.data?.key) {
    return { ok: false, error: 'Could not load push key from server.' };
  }

  const subscription = await registration.pushManager.subscribe({
    userVisibleOnly: true,
    applicationServerKey: urlBase64ToUint8Array(keyRes.data.key),
  });

  const sub = subscription.toJSON();
  const res = await post('/api/push_subscribe', {
    endpoint: sub.endpoint,
    keys: { p256dh: sub.keys.p256dh, auth: sub.keys.auth },
  });

  if (!res.ok) {
    return { ok: false, error: res.error ?? 'Failed to save subscription.' };
  }
  return { ok: true };
}
```

Save to `src/app/static/js/lib/push.js`.

- [ ] **Step 2: Verify the file is syntactically valid**

```bash
cd /Users/crispychris/Desktop/repos/homelab/src/app/static/js/lib && node --check push.js 2>&1 || python3 -c "print('node not available — will verify via browser import in Task 11 instead')"
```

Expected: either a clean syntax check, or (since this project has no Node.js runtime dependency per `CLAUDE.md`) a note that it'll be verified when the browser actually imports it in Task 11 — either outcome is fine, this step is just a fast sanity pass.

- [ ] **Step 3: Commit**

```bash
cd /Users/crispychris/Desktop/repos/homelab && git add src/app/static/js/lib/push.js && git commit -m "feat: add shared push notification frontend helper"
```

---

### Task 11: Wire `home.html` — manifest, icons, and Enable Notifications button

**Files:**
- Modify: `src/app/static/pages/home.html`
- Modify: `src/app/static/js/home.js`

**Interfaces:**
- Consumes: `registerServiceWorker`, `enablePushNotifications` from `static/js/lib/push.js` (Task 10)

- [ ] **Step 1: Add manifest/icon/meta tags to `home.html`'s `<head>`**

In `src/app/static/pages/home.html`, replace:

```html
  <link rel="stylesheet" href="/static/css/style.css">
</head>
```

with:

```html
  <link rel="stylesheet" href="/static/css/style.css">
  <link rel="manifest" href="/static/manifest.json">
  <link rel="apple-touch-icon" href="/static/icons/icon-180.png">
  <meta name="apple-mobile-web-app-capable" content="yes">
  <meta name="theme-color" content="#2b2620">
</head>
```

- [ ] **Step 2: Add the Enable Notifications button to `home.html`**

In `src/app/static/pages/home.html`, add this new `<section>` directly after the closing `</section>` of the "Upcoming" block (right before the closing `</div>` of the two-column grid, i.e. after line 99's `</section>` and before line 100's `</div>`):

```html
      <section aria-labelledby="push-heading" class="border border-hairline bg-surface p-5">
        <h2 id="push-heading" class="text-xs tracking-widest text-ink-dim uppercase mb-4">Notifications</h2>
        <button id="enable-push-btn" type="button" class="px-4 py-2 border border-accent text-accent text-xs uppercase tracking-wide hover:bg-accent hover:text-canvas transition-colors">
          Enable Notifications
        </button>
        <p id="push-status" class="text-sm text-ink-dim mt-2 hidden"></p>
      </section>
```

- [ ] **Step 3: Wire the button in `home.js`**

Add this import at the top of `src/app/static/js/home.js` (after the existing `api.js` import on line 1):

```js
import { registerServiceWorker, enablePushNotifications } from '/static/js/lib/push.js';
```

Add this near the bottom of `src/app/static/js/home.js`, right before the final `async function init() { ... }` / `init();` lines:

```js
// ---- Push notifications ----
registerServiceWorker();

const enablePushBtn = document.getElementById('enable-push-btn');
const pushStatusEl = document.getElementById('push-status');

enablePushBtn.addEventListener('click', async () => {
  enablePushBtn.disabled = true;
  pushStatusEl.classList.add('hidden');
  const res = await enablePushNotifications();
  enablePushBtn.disabled = false;
  pushStatusEl.classList.remove('hidden');
  pushStatusEl.textContent = res.ok ? 'Notifications enabled.' : (res.error ?? 'Something went wrong.');
});
```

- [ ] **Step 4: Verify the page loads with no console errors**

```bash
python3 -m venv /tmp/pwenv-verify && source /tmp/pwenv-verify/bin/activate && pip install --quiet playwright && playwright install chromium
python3 << 'EOF'
from playwright.sync_api import sync_playwright

with sync_playwright() as p:
    browser = p.chromium.launch()
    page = browser.new_page()
    errors = []
    page.on("pageerror", lambda exc: errors.append(str(exc)))
    page.on("console", lambda msg: errors.append(msg.text) if msg.type == "error" else None)
    page.goto("http://localhost:1234/static/pages/home.html", wait_until="networkidle")
    page.wait_for_timeout(500)
    print("errors:", errors)
    print("button visible:", page.locator("#enable-push-btn").is_visible())
    browser.close()
EOF
```

Expected: `errors: []`, `button visible: True`.

- [ ] **Step 5: Manual verification on the real iPhone**

This is the step that can't be automated (Apple's push relay isn't fakeable locally):
1. Delete the existing home-screen shortcut (it was added before the manifest existed, so iOS cached the old, non-installable bookmark behavior).
2. In Safari, navigate to the app's real URL, tap Share → "Add to Home Screen" again.
3. Open the app from the new home-screen icon (not from a regular Safari tab — the install context matters).
4. Go to the dashboard, tap "Enable Notifications", grant permission when prompted.
5. Create a reminder with `remind_at` a couple of minutes in the future.
6. Lock the phone or switch apps, and wait for the notification to arrive within about a minute of the due time.

Report back whether the notification arrived; if not, check `docker compose logs app` on the server for `push: send failed ...` lines, which will show the actual HTTP status/error from Apple's push relay.

- [ ] **Step 6: Commit**

```bash
cd /Users/crispychris/Desktop/repos/homelab && git add src/app/static/pages/home.html src/app/static/js/home.js && git commit -m "feat: add Enable Notifications button to dashboard"
```

---

### Task 12: Wire the remaining five pages

**Files:**
- Modify: `src/app/static/pages/reminders.html`, `src/app/static/pages/todos.html`, `src/app/static/pages/journal.html`, `src/app/static/pages/codex.html`, `src/app/static/pages/bookmarks.html`, `src/app/static/pages/logger.html`
- Modify: `src/app/static/js/reminders.js`, `src/app/static/js/todos.js`, `src/app/static/js/journal.js`, `src/app/static/js/codex.js`, `src/app/static/js/bookmarks.js`, `src/app/static/js/logger.js`

So the site behaves as one installed app regardless of which page is open when a reminder fires — the manifest/service-worker registration must be present everywhere, not just on `home.html` (only the *button* is dashboard-only, per the design).

- [ ] **Step 1: Add the same head tags to each of the six pages**

In each of the six `.html` files listed above, replace:

```html
  <link rel="stylesheet" href="/static/css/style.css">
</head>
```

with:

```html
  <link rel="stylesheet" href="/static/css/style.css">
  <link rel="manifest" href="/static/manifest.json">
  <link rel="apple-touch-icon" href="/static/icons/icon-180.png">
  <meta name="apple-mobile-web-app-capable" content="yes">
  <meta name="theme-color" content="#2b2620">
</head>
```

(Identical block to Task 11 Step 1 — every page currently duplicates this same head structure with only the `<title>` differing.)

- [ ] **Step 2: Register the service worker in each of the six JS modules**

In each of the six `.js` files listed above, add this import directly after the existing `import { ... } from '/static/js/lib/api.js';` line at the top of the file, and call it immediately:

```js
import { registerServiceWorker } from '/static/js/lib/push.js';
registerServiceWorker();
```

- [ ] **Step 3: Verify all six pages load with no console errors**

```bash
source /tmp/pwenv-verify/bin/activate 2>/dev/null || { python3 -m venv /tmp/pwenv-verify && source /tmp/pwenv-verify/bin/activate && pip install --quiet playwright && playwright install chromium; }
python3 << 'EOF'
from playwright.sync_api import sync_playwright

pages = ["reminders", "todos", "journal", "codex", "bookmarks", "logger"]

with sync_playwright() as p:
    browser = p.chromium.launch()
    for name in pages:
        page = browser.new_page()
        errors = []
        page.on("pageerror", lambda exc: errors.append(str(exc)))
        page.on("console", lambda msg: errors.append(msg.text) if msg.type == "error" else None)
        page.goto(f"http://localhost:1234/static/pages/{name}.html", wait_until="networkidle")
        page.wait_for_timeout(500)
        print(name, "errors:", errors)
        page.close()
    browser.close()
EOF
```

Expected: `errors: []` for all six pages.

- [ ] **Step 4: Commit**

```bash
cd /Users/crispychris/Desktop/repos/homelab && git add src/app/static/pages/reminders.html src/app/static/pages/todos.html src/app/static/pages/journal.html src/app/static/pages/codex.html src/app/static/pages/bookmarks.html src/app/static/pages/logger.html src/app/static/js/reminders.js src/app/static/js/todos.js src/app/static/js/journal.js src/app/static/js/codex.js src/app/static/js/bookmarks.js src/app/static/js/logger.js && git commit -m "feat: register service worker and PWA manifest on all pages"
```

---

### Task 13: Production deploy

**Files:** none (deploy-only — mirrors the Codex migration precedent from the previous session)

- [ ] **Step 1: Merge/push the branch**

```bash
cd /Users/crispychris/Desktop/repos/homelab && git checkout main && git merge feature/ios-push-notifications && git push origin main
```

- [ ] **Step 2: Migrate the production database schema**

Production's `data/` directory is excluded from `sync.sh` by design (keeps dev/prod databases separate), so the schema change from Task 1 must be applied to production's database directly, the same way the Codex folder migration was done in the previous session:

```bash
ssh chris@theonewhocentres 'cd ~/repos/homelab && docker compose stop app'
ssh chris@theonewhocentres 'cd ~/repos/homelab && cp data/app.db data/app.db.pre-push-migration-bak'
scp chris@theonewhocentres:~/repos/homelab/data/app.db /tmp/prod-app.db
python3 -c "
import sqlite3
con = sqlite3.connect('/tmp/prod-app.db')
con.execute('ALTER TABLE reminders ADD COLUMN notified_at DATETIME')
con.execute('''
CREATE TABLE push_subscriptions (
    id INTEGER PRIMARY KEY,
    endpoint TEXT NOT NULL UNIQUE,
    p256dh TEXT NOT NULL,
    auth TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
''')
con.commit()
cur = con.execute('PRAGMA table_info(reminders)')
print(cur.fetchall())
"
scp /tmp/prod-app.db chris@theonewhocentres:~/repos/homelab/data/app.db
rm /tmp/prod-app.db
```

Expected: the printed `PRAGMA table_info(reminders)` output includes the new `notified_at` column.

- [ ] **Step 3: Sync code and restart**

```bash
cd /Users/crispychris/Desktop/repos/homelab && ./sync.sh
```

Expected: rsync completes, `docker compose up -d --build` reports `homelab-app-1` recreated/started. Since `.env` is not excluded from `sync.sh`, the `VAPID_PUBLIC_KEY`/`VAPID_PRIVATE_KEY`/`VAPID_SUBSCRIBER`/`TZ` values from Task 3 are carried over automatically — no manual prod `.env` edit needed.

- [ ] **Step 4: Verify production is healthy**

Check over SSH against the server's own localhost, same as the verification pattern used for the Codex migration deploy earlier — no public URL is recorded in this repo (the app is reached through whatever `/launch`'s Cloudflare Tunnel or local network path you're already using; this step doesn't need to know it):

```bash
ssh chris@theonewhocentres 'curl -s -o /dev/null -w "home: %{http_code}\n" http://localhost:1234/static/pages/home.html'
ssh chris@theonewhocentres 'curl -s http://localhost:1234/api/push_public_key'
ssh chris@theonewhocentres 'cd ~/repos/homelab && docker compose logs app --tail 20'
```

Expected: `home: 200`; `push_public_key` returns `{"ok":true,"data":{"key":"..."}}` (proving the VAPID env vars made it to prod); no "VAPID_PUBLIC_KEY/VAPID_PRIVATE_KEY not set" warning or panics in the logs.

- [ ] **Step 5: Real-device check**

Repeat Task 11 Step 5 (delete + re-add the home-screen shortcut, enable notifications, create a near-future reminder, lock the phone, wait) against whatever URL the phone currently uses to reach the app. This is the actual acceptance test for the whole feature.

---

## Self-Review Notes

- **Spec coverage:** one-shot notify (Task 5/7) ✓, 1-minute interval (Task 7) ✓, dashboard button placement (Task 11) ✓, generated icon matching brand mark (Task 8) ✓, `push_subscriptions` table + `notified_at` column (Task 1) ✓, VAPID keys in `.env` with graceful-degrade if missing (Task 3/7) ✓, root-scoped service worker (Task 9) ✓, site-wide manifest (Task 12) ✓, stale-subscription self-cleanup (Task 7) ✓, production deploy mirroring the Codex precedent (Task 13) ✓. The timezone handling (Task 3/5) was a correctness gap found during planning, not in the original spec — added because without it the scheduler would fire notifications at the wrong real-world time (off by the server/phone UTC offset); confirmed with the user (`TZ=America/New_York`) before finalizing.
- **Placeholder scan:** no TBD/TODO markers; every code step is complete, runnable code.
- **Type consistency:** `PushSubscription{Endpoint, P256dh, Auth string}` (Task 4) matches the JSON field names decoded in `PushSubscribePOST` (Task 6) and the `webpush.Keys{P256dh, Auth}` fields consumed in the scheduler (Task 7). `registerServiceWorker`/`enablePushNotifications` (Task 10) are the exact two names imported in Tasks 11 and 12 — no drift.

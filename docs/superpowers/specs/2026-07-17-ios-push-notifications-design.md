# iOS Push Notifications for Reminders

## Problem

The homelab app is added to the user's iPhone home screen as a shortcut, but
that's just Safari's "Add to Home Screen" bookmark behavior (full-screen
webview, no address bar) — it carries zero notification capability. Reminders
already display as overdue (styled red) in-app per SEED.md's original scope
("No push notifications ... this app has no bot/automation layer"), but there
is no way to be notified while the app isn't open.

The J.A.R.V.I.S. Discord bot / `notifications` table some earlier
brainstorming considered belongs to the legacy PHP app and was explicitly
excluded from this rebuild (SEED.md migration notes) — it isn't available
here and isn't a dependency of this feature.

## Decision: real Web Push via installed PWA, one-shot per reminder

iOS Safari has supported the standard Web Push API for installed
(home-screen) web apps since iOS 16.4 — no APNs certificate, no third-party
service. This is the native mechanism; Discord/ntfy webhooks were considered
and rejected earlier in favor of this because the goal is a native-feeling
notification on the shortcut itself, not a notification in a different app.

Recurring reminders (`recurrence_type`/`recurrence_days`) are stored today
but never auto-advanced anywhere in the app — `remind_at` just stays in the
past until manually edited (that's how the existing red/overdue styling
works). This feature matches that behavior exactly: **one push per reminder,
the first time `remind_at` passes**, no recurrence-advancing engine. Building
real recurrence resolution is a separate, larger feature explicitly out of
scope here.

## Architecture

No new process, no crontab. A `time.NewTicker(1 * time.Minute)` goroutine
started in `main.go` alongside the existing `http.ListenAndServe`, sharing
the app's existing `readDB`/`writeDB`/`appCache`. This was chosen over a
separate cron-triggered binary (extra deployment step and failure surface
for no benefit at this scale) and over piggybacking on existing API requests
like `/api/reminders` (rejected outright — wouldn't fire while the phone's
screen is off and nobody has the app open, defeating the point of push).

Sending uses `github.com/SherClockHolmes/webpush-go`, which implements VAPID
JWT signing and RFC 8291 payload encryption (ECDH + HKDF + AES-128-GCM)
correctly. Hand-rolling that crypto (considered and rejected — a C
implementation was discussed) risks subtle, silent-failure bugs like
auth-tag placement or padding-byte errors.

```
┌─────────────┐   subscribe    ┌──────────────┐   1/min poll   ┌──────────┐
│  home.html   │───────────────▶│ /api/push_   │◀───────────────│ ticker   │
│  (button)    │                │ subscribe    │                │ goroutine│
└──────┬───────┘                └──────────────┘                └────┬─────┘
       │ registers                     │ stores                      │ reads due
       ▼                               ▼ subscription                ▼ reminders
┌─────────────┐                 ┌──────────────┐                ┌──────────┐
│  sw.js       │◀── push event ──│ webpush-go   │◀── sends to ───│ reminders│
│ (service     │   (encrypted)   │ (VAPID sign +│    each stored │  table   │
│  worker)     │                 │  encrypt)    │    subscription│          │
└─────────────┘                 └──────────────┘                └──────────┘
```

## Data model

Via `execute_sql`, per project convention (Golden Recipe — Database First):

```sql
ALTER TABLE reminders ADD COLUMN notified_at DATETIME;
-- NULL until a push has been sent for this reminder. Never reset — one-shot.

CREATE TABLE push_subscriptions (
    id INTEGER PRIMARY KEY,
    endpoint TEXT NOT NULL UNIQUE,
    p256dh TEXT NOT NULL,
    auth TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

## VAPID keys

Generated once (via a short-lived script using `webpush.GenerateVAPIDKeys()`),
stored as `VAPID_PUBLIC_KEY` / `VAPID_PRIVATE_KEY` in `.env` — the same file
`SESSION_SECRET` already lives in, which already flows to production through
`sync.sh` (not excluded, unlike `data/`). Unlike `SESSION_SECRET` (which
`log.Fatal`s if missing, since every request needs a session), missing VAPID
keys only log a warning and skip starting the push ticker — push is
additive, not core to the app functioning.

## Backend additions

- `handlers/push_subscribe.go` — `POST /api/push_subscribe`, body
  `{endpoint, keys: {p256dh, auth}}`, `INSERT OR IGNORE` on `endpoint` so
  re-subscribing the same device is a no-op, not a duplicate row
- `handlers/push_public_key.go` — `GET /api/push_public_key`, returns
  `{"key": "<VAPID public key>"}` for the frontend to pass into
  `pushManager.subscribe()`
- `r.Get("/sw.js", ...)` in `main.go` — serves `static/sw.js` from the
  document root (not under `/static/`) so the service worker's default scope
  covers the entire site with no extra `Service-Worker-Allowed` header needed
- Ticker goroutine in `main.go`: every minute, queries
  `reminders WHERE is_active = 1 AND remind_at <= now AND notified_at IS NULL`;
  for each due reminder, sends to every stored subscription, then marks
  `notified_at` regardless of per-subscription send outcome (a bad
  subscription must not block the reminder from ever being marked handled);
  a `410 Gone` / `404` response from a subscription deletes that row
  (expired installs self-clean)

## Frontend additions

- `static/manifest.json` — `display: standalone`, `start_url: "/"`, icons
  (see below)
- `static/sw.js` — minimal service worker: `push` event calls
  `self.registration.showNotification(title, {body, icon})`;
  `notificationclick` focuses/opens the app
- `static/js/lib/push.js` — shared helper: registers the service worker,
  requests `Notification` permission, subscribes, POSTs to
  `/api/push_subscribe`
- "Enable Notifications" button on `home.html`, calling the helper above and
  showing an inline status (Enabled / Blocked — check Settings / error)
- `<link rel="manifest">`, `apple-mobile-web-app-capable` meta tag, and icon
  `<link>`s added to every page's `<head>` (all six pages currently duplicate
  this header block) so the site behaves as one installed app regardless of
  which page is open when a reminder fires
- New icon: a simple generated SVG/PNG matching the existing brand mark (the
  pulsing accent-color dot + "HOMELAB" wordmark already in the header) — no
  new visual language. 192×192 and 512×512 PNGs for the manifest, reused as
  the notification icon.

## Error handling

- `/api/push_subscribe` returns a clear JSON error if required fields are
  missing (matches existing handler conventions — `jsonError`)
- The ticker goroutine logs and continues on individual send failures;
  `chiMiddleware.Recoverer` (already registered) guards against a panic in
  the loop taking down the server
- Permission-denied or unsupported-browser cases are handled client-side
  with a plain inline status message — no dead ends, no silent failure

## Testing

- Playwright drives the subscribe flow (button click → permission grant in
  a test context → POST fires) and asserts the `push_subscriptions` row
  appears
- The ticker loop is verified by inserting a reminder with a past
  `remind_at`, waiting for a tick, and confirming `notified_at` gets set and
  a subscription's mock endpoint receives the request
- Actual delivery to a real iPhone can't be automated (Apple's push relay
  isn't fakeable locally) — verified manually against the user's phone as
  the final acceptance check

## Out of scope

- Recurrence-aware re-notification (separate future feature)
- Multi-user / per-user subscription scoping (this is a single-user personal
  app; the subscribe endpoint is unauthenticated, matching the rest of the
  app's security model)
- Notification action buttons (e.g. "Mark done" from the notification itself)

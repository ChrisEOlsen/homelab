# Finances — Design

## Goal

A finances page for Homelab that turns the work calendar into an income ledger and
subtracts manually-entered spending from it, so the month's net is visible at a
glance. Income is derived from the `calsync` calendar feed; spending is two manual
lists (shopping items, recurring subscriptions). The ledger is permanent — the
calendar feed is a rolling six-week window, but every session that ever passes
through it is kept forever.

## Context

### The calendar source (verified on the server, 2026-08-15)

`calsync` lives at `chris@theonewhocentres:~/custom-skills/personal-productivity`.
It is a systemd **user** service (`calsync.service`, enabled, up ~3 weeks) that
scrapes two booking apps every 10 minutes with headless Chromium and writes one
merged `calendar.ics` (128 events at time of writing), served over HTTP.

- `HOST=127.0.0.1,100.85.186.108` — loopback plus this box's Tailscale IP, port `5522`.
- **The Homelab app container can already reach it.** Verified from inside
  `homelab-app-1`: `curl http://100.85.186.108:5522/calendar.ics` → `200`, 62298 bytes.
  No new plumbing, no bind mount — one env var.

Two sources, and they split exactly along the pricing line:

| `X-CALSYNC-SOURCE` | App | `SUMMARY` shape | Pricing |
|---|---|---|---|
| `wl` | WellnessLiving (the gym) | `Patrick Sinclair — 45-Minute Training` | by duration |
| `cc` | coachchrisfitness.com (own app) | `John Kublacki` (no service) | independent, $100 |

Event structure per `lib/ics.js`: `UID`, `SUMMARY` (`client — service`, joined with
an em-dash; `cc` has no service so the summary is the bare client name),
`DTSTART;TZID=America/New_York`, `DTEND`, `DESCRIPTION`, `X-CALSYNC-SOURCE`,
optional `X-CALSYNC-ID`, optional `URL`. Lines are folded at 75 octets per RFC 5545.

### Three constraints that drive the design

1. **The feed is a rolling window** — current week plus `WEEKS_AHEAD=5` weeks
   forward. Past sessions disappear from it. The ledger therefore cannot be a
   mirror of the feed; it must be write-once-and-keep. It also means history
   begins at first sync (~2026-08-10); earlier months require manual entry.
2. **`cc` events have no stable ID.** `sources/ccfitness.js` sets `id: null` and
   `lib/ics.js` derives the UID from date+client. A reschedule therefore looks
   like a brand-new event, so sync needs real reconciliation, not blind upsert.
3. **Not every event is a paid session.** The feed currently contains
   `(8/03-9/06) Women's Strength: Summer Skill Series`, a 60-minute WL block that
   would otherwise price at $60.

### Existing app conventions this follows

- Pages are plain static files under `static/pages/`, linked as
  `/static/pages/<name>.html`. Header/nav markup is duplicated verbatim in every
  `.html`; nav-toggle/clock wiring is duplicated in every `.js`.
- All handlers return JSON via `handlers/json.go` helpers; routes come from
  `api.json` → `routes_gen.go`; models own all SQL.
- `models.Time` for timestamps, integer money, `id INTEGER PRIMARY KEY`.
- The live database is only on the server; the repo's `data/` is empty.

## Decisions

| Question | Decision |
|---|---|
| Planned (unbought) shopping items | Do **not** reduce net. Shown as a separate "committed" figure, plus a derived "net after committed" line so the effect is visible without distorting the headline. |
| `cc` session whose client isn't in the table | Prices at **$100** and is flagged for review. Everything on his own booking app is an independent client, so income stays correct even if the review strip is never opened. |
| Gym duration rates ($45/$50/$60) | An **editable `rate_rules` table**, not constants. A rate change is a form edit, not a redeploy. |
| Name matching | **Exact full name**, case/whitespace/unicode-normalized. Never substring, never surname. Ofer Rubin (independent, $100) is Ran Rubin's son; Ran is a gym client priced by duration. A surname match would silently inflate income. |

## Data model

Six tables. All money is integer cents. All dates are `YYYY-MM-DD` strings in
America/New_York local time, so month grouping never shifts across a UTC boundary.

```sql
CREATE TABLE clients (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,                              -- display name
    match_name TEXT NOT NULL,                        -- exact name as it appears in the calendar
    email TEXT,
    phone TEXT,
    rate_cents INTEGER NOT NULL DEFAULT 10000,
    kind TEXT NOT NULL DEFAULT 'independent',        -- independent | ignored
    is_active INTEGER NOT NULL DEFAULT 1,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX idx_clients_match_name ON clients(match_name COLLATE NOCASE);

CREATE TABLE rate_rules (
    id INTEGER PRIMARY KEY,
    duration_min INTEGER NOT NULL UNIQUE,
    amount_cents INTEGER NOT NULL,
    label TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Named training_sessions, not sessions: models.Session would sit next to the
-- auth session cookie helpers in middleware/ and read as the same concept.
CREATE TABLE training_sessions (
    id INTEGER PRIMARY KEY,
    uid TEXT NOT NULL UNIQUE,                        -- ICS UID; synthetic for manual rows
    source TEXT NOT NULL,                            -- wl | cc | manual
    client_name TEXT NOT NULL,                       -- as it appeared in the calendar
    client_id INTEGER REFERENCES clients(id) ON DELETE SET NULL,
    service TEXT,
    session_date TEXT NOT NULL,                      -- YYYY-MM-DD
    start_at TEXT NOT NULL,                          -- YYYY-MM-DD HH:MM:SS, NY wall clock
    end_at TEXT NOT NULL,
    duration_min INTEGER NOT NULL,
    amount_cents INTEGER NOT NULL DEFAULT 0,
    rate_source TEXT NOT NULL DEFAULT 'unknown',     -- override | client | rule | unknown
    override_cents INTEGER,
    status TEXT NOT NULL DEFAULT 'scheduled',        -- scheduled | cancelled | ignored
    needs_review INTEGER NOT NULL DEFAULT 0,
    first_seen_at TEXT,                              -- TEXT, not DATETIME: see note below
    last_seen_at TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_training_sessions_date ON training_sessions(session_date);
CREATE INDEX idx_training_sessions_status ON training_sessions(status);

CREATE TABLE expenses (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    amount_cents INTEGER NOT NULL,
    category TEXT,
    status TEXT NOT NULL DEFAULT 'planned',          -- planned | bought
    incurred_on TEXT NOT NULL,                       -- YYYY-MM-DD, the month it counts against
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_expenses_incurred_on ON expenses(incurred_on);

CREATE TABLE subscriptions (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    amount_cents INTEGER NOT NULL,
    cadence TEXT NOT NULL DEFAULT 'monthly',         -- monthly | yearly | weekly
    billing_day INTEGER,
    is_active INTEGER NOT NULL DEFAULT 1,
    started_on TEXT NOT NULL,
    ended_on TEXT,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE calendar_syncs (
    id INTEGER PRIMARY KEY,
    finished_at TEXT,
    ok INTEGER NOT NULL DEFAULT 0,
    events_seen INTEGER NOT NULL DEFAULT 0,
    created_count INTEGER NOT NULL DEFAULT 0,
    updated_count INTEGER NOT NULL DEFAULT 0,
    cancelled_count INTEGER NOT NULL DEFAULT 0,
    error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP   -- doubles as started_at
);
```

**Two generator constraints the DDL above already satisfies**, both discovered by
reading `src/builder/`:

- **Every table needs a `created_at` column.** `model.go.tmpl` always selects and
  scans `created_at` into `models.Time`; a table without it generates a model that
  fails at runtime.
- **Any timestamp column declared as a scaffold field must be `TEXT`, not
  `DATETIME`.** `schema.go`'s `applySchema` compares the real column's normalized
  affinity against `expectedSQLType`, and `DATETIME` normalizes to `DATETIME`,
  which does not equal the `TEXT` a `:string` field expects — the scaffold call
  fails outright. `created_at` is exempt because it is never a declared field.
  Columns not declared as fields are never validated, so they may be `DATETIME`.

**Seed data** (part of the migration):

- `clients`: Adam Schwarzschild (adam.schwarzschild@gmail.com, 4044412313),
  Susie Kim (susieahnkim@gmail.com, 9178808738), Puneet Riverside
  (puneet@assigned.com), John Kublacki (john@assigned.com), Ofer Rubin — all
  `kind='independent'`, `rate_cents=10000`, `match_name` = name.
  Ofer is currently on vacation and has no sessions in the window; he starts
  producing rows as soon as he books again.
- `rate_rules`: `30 → 4500`, `45 → 5000`, `60 → 6000`.

**`kind='ignored'`** is how a non-client calendar entry is excluded permanently:
add `(8/03-9/06) Women's Strength: Summer Skill Series (stand-alone …)` as a client
row with `kind='ignored'`, and every matching event — past, present, and future
re-syncs — is skipped. One CRUD table, no special-case code.

### Two rules that must not be ambiguous

- **Marking an expense `bought` sets `incurred_on` to today**, so a month's net
  reflects when money actually left. The field stays editable if the real purchase
  date differs.
- **Deactivating a subscription sets `ended_on` to today** if it is null. Month
  membership is then purely a date-range question: a subscription counts in month
  *M* if `started_on <= end(M)` and (`ended_on IS NULL` or `ended_on >= start(M)`).
  `is_active` is a UI affordance, never a term in the month query.

## Rate resolution

A pure function over `(session, clients, rate_rules)`, so it is trivially testable:

1. `override_cents` set → use it, `rate_source='override'`.
2. Exact normalized match on a client's `match_name` (fall back to `name`):
   - `kind='ignored'` → `status='ignored'`, amount 0, excluded from all totals.
   - otherwise → `client.rate_cents`, `rate_source='client'`.
3. No client match, `source='cc'` → `10000`, `rate_source='client'`,
   `needs_review=1` (a new independent client).
4. No client match, `source='wl'` → exact `duration_min` hit in `rate_rules` →
   that amount, `rate_source='rule'`.
5. Anything else (odd duration, e.g. a 90-minute block) → amount 0,
   `rate_source='unknown'`, `needs_review=1`.

Normalization is trim + collapse internal whitespace + Unicode NFC + case-fold.
Nothing wider than that — see the Rubin case above.

## Calendar sync

New hand-written package `src/app/calendar/` (infrastructure — no MCP scaffold tool
applies, same category as `middleware/` and `db/`):

- **`ics.go`** — RFC 5545 line unfolding and VEVENT extraction. No new Go
  dependency; the subset in play is small and fully determined by `lib/ics.js`.
  Splits `SUMMARY` on the em-dash into client and service; a summary with no
  em-dash is all client, no service. Duration comes from `DTEND − DTSTART`, not
  from parsing "45-Minute Training" out of the service text.
- **`rates.go`** — the resolution function above.
- **`sync.go`** — fetch, upsert, reconcile, log. Uses model methods only.

**The sync algorithm:**

1. `GET` `CALENDAR_ICS_URL` with an 15s timeout. On any error: write a failed
   `calendar_syncs` row and return — the ledger is untouched.
2. Parse. **If zero events parsed, stop before reconciliation.** A bad scrape must
   never be able to wipe a month. (`calsync` has its own anti-flap guard upstream;
   this is the second belt.)
3. Compute the window this feed actually covers: `[min(DTSTART), max(DTSTART)]`.
4. Upsert each event by `uid`. On update, refresh the calendar-derived fields and
   `last_seen_at`, but **preserve every manual decision** — `override_cents`,
   `status='ignored'`, and any hand-edited `client_id`.
5. Reconcile: any stored session whose `session_date` falls **inside** the window
   but whose `uid` was not seen this run → `status='cancelled'`. Kept as a row,
   excluded from income. This is what makes reschedules and cancellations correct,
   including the `cc` case where a reschedule appears as a new UID.
6. Sessions dated **before** the window are frozen and never touched. That is the
   permanent history.
7. Nothing is ever deleted by sync.

**Triggers:** a `Sync now` button (`POST /api/v1/calendar/sync`) and a background
ticker in `main.go` every `CALENDAR_SYNC_INTERVAL_MIN` minutes (default 30, `0`
disables). A mutex ensures the two can't overlap.

**New env vars** (`.env` on the server, plus `env.example`):

```
CALENDAR_ICS_URL=http://100.85.186.108:5522/calendar.ics
CALENDAR_SYNC_INTERVAL_MIN=30
```

## API surface

Built with MCP scaffold tools per the mandatory scaffolding rule:

| Tool | Builds |
|---|---|
| `scaffold_resource` | `client`, `rate_rule`, `training_session`, `expense`, `subscription` — full CRUD, sort/filter, self-registered |
| `create_model` | `calendar_sync` — model only; rows are written by the sync service and read by the summary endpoint |
| `create_handler` | `calendar_sync` → `POST /api/v1/calendar/sync` |
| `create_page` | `finances` → `.html` shell + `.js` module + a `FinancesGET` handler stub registered at `GET /api/v1/finances/summary`, which **is** the summary endpoint |

`training_session` gets full CRUD so a session can be manually added (backfilling
months older than the feed window), overridden, or corrected.

Three generator behaviours this mapping accounts for:

- **`create_page` requires a path under `/api/v1/`** (`main.go` rejects anything
  else) and its generated Go handler is a JSON stub, not an HTML server — the page
  itself is served as a static file at `/static/pages/finances.html`, exactly like
  the six existing pages. So the page's own generated handler becomes the summary
  endpoint; no second `create_handler` call is needed.
- **`scaffold_resource` also emits a list page** (`static/pages/<plural>.html` +
  `static/js/<plural>.js`) that no route references. All UI lives on the single
  finances page, so each resource task deletes its surplus generated pair. The five
  API routes it registers are unaffected.
- **`create_model` emits no page and no route**, which is what `calendar_sync`
  wants.

**`GET /api/v1/finances/summary?month=YYYY-MM`** (defaults to the current month),
returned via `jsonOK` — it is a single object, not a list:

```json
{
  "ok": true,
  "data": {
    "month": "2026-08",
    "income": {
      "earned_cents": 0,
      "projected_cents": 0,
      "session_count": 0,
      "by_source": { "wl": 0, "cc": 0, "manual": 0 }
    },
    "spending": {
      "subscriptions_cents": 0,
      "shopping_bought_cents": 0,
      "shopping_committed_cents": 0
    },
    "net_cents": 0,
    "net_after_committed_cents": 0,
    "all_time": { "income_cents": 0, "spend_cents": 0, "net_cents": 0 },
    "needs_review_count": 0,
    "unmatched_names": [],
    "last_sync": { "finished_at": "...", "ok": true, "events_seen": 128 },
    "sessions": [],
    "expenses": []
  }
}
```

`sessions` and `expenses` carry the selected month's rows inline. The resource list
endpoints filter by equality only (`?filter=col:value`), so a month is not
expressible there without fetching everything; and the page wants the totals and
the rows in the same paint anyway. One request drives the whole page; the
`clients`, `rate_rules`, and `subscriptions` panels use their own list endpoints.

Definitions:

- `earned_cents` — sessions in the month with `status='scheduled'` and `end_at <= now`.
- `projected_cents` — all `status='scheduled'` sessions in the month, past and future.
- `subscriptions_cents` — monthly-equivalent sum: monthly → amount, yearly →
  `round(amount/12)`, weekly → `round(amount*52/12)`.
- `net_cents` = `earned − subscriptions − shopping_bought`.
- `net_after_committed_cents` = `net_cents − shopping_committed`.
- `unmatched_names` — distinct `client_name` on `needs_review=1` sessions with no
  `client_id`, for the one-click "add as client" affordance.

**Earned vs. projected matters.** Future sessions already sit in the calendar, so a
single "income" number would display the whole month's bookings on the 3rd. Both
are shown; net is computed from earned.

## Page

`static/pages/finances.html` + `static/js/finances.js`, in the existing Instrument
theme, mobile-first like the rest of the app.

- **Month bar** — `‹ August 2026 ›`, defaults to the current month.
- **Ledger strip** — Earned · Projected · Subscriptions · Shopping · **Net** (the
  emphasized figure), with committed shown as a secondary line under Shopping.
- **Sessions** — the month's sessions grouped by day: time, client, source badge
  (`gym` / `independent`), duration, amount. Review-flagged rows are marked, with
  inline override and ignore actions. `Sync now` button showing last sync time and
  a failure state if the last run errored.
- **Shopping** — add form, planned/bought toggle, delete.
- **Subscriptions** — CRUD with the normalized monthly total.
- **Clients** — CRUD over the seeded five, `kind` toggle, plus an **unmatched
  calendar names** panel driven by `unmatched_names` with one-click "add as client".
- **Rates** — the three `rate_rules` rows, editable inline.
- **All-time footer** — total income, total spend, net since the ledger began.

Nav: a `Finances` entry added to the desktop nav bar and mobile drawer in all
seven `.html` pages, plus a tile in the dashboard's nav grid. All DOM built with
`createElement`/`textContent` — no `innerHTML` for any calendar-derived value.
All fetches go through `lib/api.js`, which carries the CSRF token.

## Error handling

- Feed unreachable, HTTP non-200, or unparseable → failed `calendar_syncs` row,
  ledger untouched, page shows "last sync failed" with the timestamp of the last
  good run. Sync never returns a 500 for an upstream failure; it returns `ok: true`
  with a result body describing the failed run, so the button always gets a usable
  answer.
- Zero parsed events → treated as a failure for reconciliation purposes (step 2).
- Unknown `?month=` format → `422 validation_failed`.
- Unknown sort/filter column on any resource list → `422`, per `models/query.go`.
- A `clients` row deleted while sessions reference it → `ON DELETE SET NULL`; those
  sessions keep their recorded `amount_cents` and are re-resolved on next sync.

## Testing

Scaffolded resources generate their own model CRUD and handler tests. Hand-written
tests for everything that carries real risk:

- `calendar/ics_test.go` — folded-line unfolding, TZID parsing, `wl` summary split,
  `cc` bare-name summary, the class event with no client, malformed VEVENT skipped.
  Golden fixture of ~6 trimmed events.
- `calendar/rates_test.go` — the full resolution ladder, **including Ofer vs. Ran
  Rubin**, `kind='ignored'`, `cc` unmatched → $100 + flag, odd duration → review.
- `calendar/sync_test.go` — upsert idempotency across repeated syncs, in-window
  reconciliation marks cancelled, out-of-window rows frozen, overrides and ignores
  survive re-sync, zero-event feed changes nothing.
- `handlers/finances_test.go` — earned vs. projected split, cadence normalization,
  net formula, month boundaries.

Verified with `docker compose exec app go test ./...` plus `docker compose logs app`.

## Deployment

Work happens on branch `build/finances` in the main checkout — no worktree, per
`CLAUDE.md`. The `gova-builder` MCP server is connected and confirmed pointed at
this checkout (`inspect_app` → 12 existing models, zero divergence).

**The live database needs the schema applied separately.** `execute_sql` writes to
the local, empty `data/app.db`; the real data lives only in the server's
`data/app.db`. So:

1. The DDL and seed rows are committed as `docs/migrations/2026-08-15-finances.sql`.
2. On the server: `docker compose stop app` →
   `cp data/app.db data/app.db.pre-finances-migration-bak-$(date +%Y%m%d%H%M%S)`
   (matching the existing `.pre-*-migration-bak` convention already in `data/`) →
   apply the SQL → restart.
   Neither the host nor either container has a `sqlite3` CLI; the server's
   `python3` has the `sqlite3` module (3.45.1), which is enough.
3. Add `CALENDAR_ICS_URL` and `CALENDAR_SYNC_INTERVAL_MIN` to the server's `.env`.
4. Deploy with `./sync.sh` (push here, pull + rebuild there).

## Out of scope

Taxes and deductions; invoicing or receivables (a session is income when it
happens, not when paid); bank or card import; charts and trend lines; multi-currency;
automatic backfill of sessions predating the first sync — those are entered by hand
through the `training_session` resource.

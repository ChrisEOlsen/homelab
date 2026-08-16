# Known Limitations

Things this app does not do, and why. Written down so they are decisions rather
than surprises.

## Finances — the calendar feed is a rolling window

`calsync` scrapes a **forward-rolling window**, not a calendar month. At the time
of writing it serves roughly the current week plus five weeks ahead — on
2026-08-16 the feed held 133 events spanning **2026-08-17 → 2026-09-19**, with
nothing before the 17th.

The ledger is built for this. Sync upserts by UID, reconciles disappearances only
inside the window the feed actually covered, and **freezes everything older**
rather than reading "absent from the feed" as "cancelled". Nothing is ever
deleted.

That design is load-bearing, not theoretical: on 2026-08-16 the database held 156
sessions spanning 2026-08-10 → 2026-09-19, of which the 21 sessions on Aug 10–15
had already rolled out of the feed entirely. Those 21 were the whole of August's
earned income to that point — $1,395. A mirror-style sync would have shown $0.

The consequences below all follow from that one fact.

### 1. August 2026 is permanently incomplete

The first sync ran on 2026-08-15, when the feed already started at 2026-08-10.
**Sessions from 2026-08-01 to 2026-08-09 were never in any feed the app saw and
never will be.** August's income is understated by nine days.

This is a one-time hole caused by the app being born mid-month. Every later month
starts complete, because sessions enter the feed four to five weeks before they
happen and the sync ticker runs every 30 minutes — a session is seen on the order
of a thousand times before it rolls out.

**Workaround:** enter the missing sessions by hand. `POST /api/v1/training_sessions`
accepts a full session; give it a synthetic `uid` (anything not colliding with a
calsync UID, e.g. `manual-2026-08-04-ofer`) and `source: "manual"`. Manual rows are
deliberately excluded from reconciliation, so the sync will never cancel them.

### 2. Cancelling a *past* session does not register

Reconciliation can only act inside the window the feed covered. Outside it, the
app cannot distinguish "this was cancelled" from "this fell out of the feed" — and
guessing wrong in that direction would silently delete real income.

So: **cancel a session while it is still in the feed and the ledger follows.**
Cancel one the feed no longer covers and it stays counted as earned until you
correct it by hand.

The boundary is "still in the feed", not strictly "in the future":

- Anything from **tomorrow onward is always safe**.
- **Today may already be outside it.** On 2026-08-16 the feed's earliest event was
  the 17th, so a session later that same day was already frozen.
- The window's floor is the **earliest booked event in the feed**, not the feed's
  true coverage start. If the first day or two of the feed's range happens to have
  no bookings, the effective floor moves later, and a cancellation on one of those
  quiet days falls outside it.

This errs toward keeping income rather than wrongly deleting it, which is the
correct direction to fail.

**Workaround:** `PUT /api/v1/training_sessions/{id}` with `status: "ignored"` —
which parks the row at zero and survives every future sync. Do **not** set
`status: "cancelled"` by hand: if the session is still inside the feed window the
next sync will revert it, because the feed re-reporting an appointment is
authoritative evidence it is back on.

### 3. A long outage could lose a session

Because sessions are visible for weeks before they occur, the only real loss case
is a session **booked and occurring entirely within an app outage**. Narrow, but
the app has no way to detect that it happened.

### 4. Group classes price as ordinary sessions

A non-client calendar entry with a normal duration — e.g.
`(8/03-9/06) Women's Strength: Summer Skill Series`, 60 minutes — matches a
duration rate rule and prices as a $60 gym session. It is **not** flagged for
review, because nothing about it looks anomalous to the pricing ladder.

**Workaround:** add it as a client with `kind: "ignored"`. Every matching session,
past and future, is then zeroed and parked permanently.

### 5. Deactivating a client re-prices their history

`is_active = 0` removes a client from name matching entirely rather than archiving
them. On the next sync their sessions fall back to the independent default plus a
review flag, and an inactive `kind: "ignored"` client's sessions stop being parked
and start counting income again.

There is no UI toggle for `is_active` today, so this is latent. It is left as-is
because "what should inactive mean" is a product decision, not a bug fix.

### The structural fix, not yet done

`calsync` reads the current week and pages **forward** `WEEKS_AHEAD` weeks. Adding
a `WEEKS_BEHIND` so each scrape also walks back a week or two would make the whole
thing self-healing after an outage, let a fresh install capture recent history
instead of starting empty, and shrink the past-cancellation blind spot to whatever
window it walks back.

That is a change to `~/custom-skills/personal-productivity`, not this repo, and it
touches a scraper that currently works reliably — so it is deliberately separate
work.

## Finances — scope of the numbers

- **The ledger strip and the chart are month-scoped** and reset each month. Only
  the all-time footer accumulates.
- **Nothing carries over.** August's leftover Net does not become September's
  opening balance. The page answers "how am I doing this month", not "what is my
  balance".
- **Planned shopping never reduces Net.** It is reported as a separate committed
  figure, with a derived "net after committed" line.
- **Subscriptions recur automatically** each month, normalized to a monthly
  figure (yearly ÷ 12, weekly × 52 ÷ 12).
- **Stopping a subscription takes effect immediately**, including the month you
  stop it in — the page answers "if I stop paying this, what do I have?", so a
  stop has to move the number you are looking at. A subscription counts for a
  month only if it was still live at the end of that month, which means the last
  month it counts for is the one *before* the month it was stopped. Months it
  ran through in full are unaffected.

  The trade: stopping on the 28th still removes the whole month, even though you
  most likely paid it. If that matters, edit `ended_on` to a date in the
  following month via `PUT /api/v1/subscriptions/{id}`.

## App-wide

- **No authentication.** Single user, protected by being on the tailnet. Every API
  route is public to anything that can reach the host.
- **No JS test harness** (no Node/npm in the image, by design). Client-side code is
  verified by hand and in the browser; Go code has `go test ./...`.
- **The manifest records `create_model` models inconsistently.** `api.json` lists
  models created by `scaffold_resource`/`scaffold_list` in snake_case while the
  twelve original models are PascalCase, and models created by `create_model` are
  absent from the list entirely. Cosmetic — routing is unaffected — but it makes
  `api.json` an imperfect source of truth for a typed client. Fixing it means
  editing `src/builder/` and rebuilding the mcp image.

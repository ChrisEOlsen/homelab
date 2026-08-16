import { get, post, put, del } from '/static/js/lib/api.js';

// ---- Clock + mobile drawer (same wiring every page carries) ----
function tickClock() {
  const text = new Date().toLocaleTimeString([], { hour12: false });
  const clock = document.getElementById('clock');
  const clockMobile = document.getElementById('clock-mobile');
  if (clock) clock.textContent = text;
  if (clockMobile) clockMobile.textContent = text;
}
tickClock();
setInterval(tickClock, 1000);

const navToggle = document.getElementById('nav-toggle');
const navClose = document.getElementById('nav-close');
const drawer = document.getElementById('mobile-drawer');
const backdrop = document.getElementById('mobile-drawer-backdrop');

function openDrawer() {
  drawer.classList.remove('translate-x-full');
  backdrop.classList.remove('hidden');
  navToggle.setAttribute('aria-expanded', 'true');
}
function closeDrawer() {
  drawer.classList.add('translate-x-full');
  backdrop.classList.add('hidden');
  navToggle.setAttribute('aria-expanded', 'false');
}
navToggle.addEventListener('click', openDrawer);
navClose.addEventListener('click', closeDrawer);
backdrop.addEventListener('click', closeDrawer);
document.addEventListener('keydown', (e) => { if (e.key === 'Escape') closeDrawer(); });
drawer.querySelectorAll('a').forEach((a) => a.addEventListener('click', closeDrawer));

// ---- Elements ----
const monthLabelEl = document.getElementById('month-label');
const ledgerEl = document.getElementById('ledger');
const sessionsEl = document.getElementById('sessions-panel');
const allTimeEl = document.getElementById('all-time');
const syncBtn = document.getElementById('sync-btn');
const syncStatusEl = document.getElementById('sync-status');

// ---- State ----
export const state = {
  month: new Date().toISOString().slice(0, 7), // YYYY-MM in local terms is close
  summary: null,
  clients: [],
  rateRules: [],
  subscriptions: [],
};

// ---- Money and date helpers ----
// Everything on the wire is integer cents; only this formatter turns them into
// currency, and only for display.
export function fmtMoney(cents) {
  const n = Number(cents ?? 0) / 100;
  return n.toLocaleString(undefined, { style: 'currency', currency: 'USD' });
}

// parseMoney accepts "45", "$45", "45.50" and returns integer cents. Anything
// unparseable returns null so the caller can show a field error.
export function parseMoney(text) {
  const cleaned = String(text ?? '').replace(/[$,\s]/g, '');
  if (cleaned === '' || !/^\d*\.?\d*$/.test(cleaned)) return null;
  const n = Number(cleaned);
  if (!isFinite(n)) return null;
  return Math.round(n * 100);
}

function monthLabel(month) {
  const [y, m] = month.split('-').map(Number);
  return new Date(y, m - 1, 1).toLocaleDateString(undefined, { month: 'long', year: 'numeric' });
}

function shiftMonth(month, delta) {
  const [y, m] = month.split('-').map(Number);
  const d = new Date(y, m - 1 + delta, 1);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}`;
}

function dayLabel(dateStr) {
  const [y, m, d] = dateStr.split('-').map(Number);
  return new Date(y, m - 1, d).toLocaleDateString(undefined, {
    weekday: 'short', month: 'short', day: 'numeric',
  });
}

function timeLabel(stamp) {
  const t = stamp.slice(11, 16);
  const [h, min] = t.split(':').map(Number);
  const d = new Date(2000, 0, 1, h, min);
  return d.toLocaleTimeString([], { hour: 'numeric', minute: '2-digit' });
}

// ---- Shared DOM builders ----
// panel() gives every section the same heading row, so the page reads as one
// instrument rather than six stacked widgets.
export function panel(container, title, subtitle) {
  container.replaceChildren();

  const head = document.createElement('div');
  head.className = 'flex items-baseline justify-between gap-3';

  const h = document.createElement('h2');
  h.className = 'text-sm font-medium text-ink';
  h.textContent = title;
  head.appendChild(h);

  if (subtitle) {
    const s = document.createElement('span');
    s.className = 'text-xs text-ink-dim tabular-nums';
    s.textContent = subtitle;
    head.appendChild(s);
  }
  container.appendChild(head);

  const body = document.createElement('div');
  body.className = 'space-y-2';
  container.appendChild(body);
  return body;
}

export function errorLine(container, message) {
  const p = document.createElement('p');
  p.className = 'text-sm text-danger';
  p.textContent = message;
  container.appendChild(p);
}

export function emptyLine(container, message) {
  const p = document.createElement('p');
  p.className = 'text-sm text-ink-dim';
  p.textContent = message;
  container.appendChild(p);
}

export function iconButton(label, onClick, danger = false) {
  const b = document.createElement('button');
  b.type = 'button';
  b.className = danger
    ? 'px-2 py-1 text-xs border border-hairline text-danger hover:bg-surface-raised transition-colors'
    : 'px-2 py-1 text-xs border border-hairline text-ink-dim hover:text-ink hover:bg-surface-raised transition-colors';
  b.textContent = label;
  b.addEventListener('click', onClick);
  return b;
}

// ---- Ledger strip ----
function tile(label, value, emphasis = false) {
  const cell = document.createElement('div');
  cell.className = 'bg-surface px-4 py-3';

  const l = document.createElement('div');
  l.className = 'text-xs text-ink-dim';
  l.textContent = label;
  cell.appendChild(l);

  const v = document.createElement('div');
  v.className = emphasis
    ? 'text-lg font-semibold tabular-nums text-ink'
    : 'text-sm tabular-nums text-ink';
  v.textContent = value;
  cell.appendChild(v);
  return cell;
}

function renderLedger() {
  const s = state.summary;
  ledgerEl.replaceChildren();
  if (!s) return;

  ledgerEl.appendChild(tile('Earned', fmtMoney(s.income.earned_cents)));
  ledgerEl.appendChild(tile('Projected', fmtMoney(s.income.projected_cents)));
  ledgerEl.appendChild(tile('Subscriptions', fmtMoney(s.spending.subscriptions_cents)));

  // Planned shopping never reduces Net — it's a secondary line under Shopping,
  // not folded into the big number.
  const shopping = tile('Shopping', fmtMoney(s.spending.shopping_bought_cents));
  const committed = document.createElement('div');
  committed.className = 'text-xs text-ink-dim tabular-nums';
  committed.textContent = `+ ${fmtMoney(s.spending.shopping_committed_cents)} planned`;
  shopping.appendChild(committed);
  ledgerEl.appendChild(shopping);

  const net = tile('Net', fmtMoney(s.net_cents), true);
  const after = document.createElement('div');
  after.className = 'text-xs text-ink-dim tabular-nums';
  after.textContent = `${fmtMoney(s.net_after_committed_cents)} after planned`;
  net.appendChild(after);
  ledgerEl.appendChild(net);
}

function renderAllTime() {
  const s = state.summary;
  allTimeEl.replaceChildren();
  if (!s) return;
  allTimeEl.textContent =
    `All time — income ${fmtMoney(s.all_time.income_cents)} · ` +
    `spend ${fmtMoney(s.all_time.spend_cents)} · ` +
    `net ${fmtMoney(s.all_time.net_cents)}`;
}

// ---- Sessions panel ----
const SOURCE_LABELS = { cc: 'independent', wl: 'gym', manual: 'manual' };

function sessionRow(item) {
  const li = document.createElement('li');
  li.className = 'flex flex-wrap items-center gap-x-3 gap-y-1 py-2 border-b border-hairline last:border-0';
  li.dataset.id = item.id;

  const time = document.createElement('span');
  time.className = 'text-xs text-ink-dim tabular-nums w-16 shrink-0';
  time.textContent = timeLabel(item.start_at);
  li.appendChild(time);

  const name = document.createElement('span');
  name.className = 'text-sm text-ink flex-1 min-w-32';
  name.textContent = item.client_name; // textContent — the feed is external data
  li.appendChild(name);

  const badge = document.createElement('span');
  badge.className = 'text-xs text-ink-dim';
  badge.textContent = `${SOURCE_LABELS[item.source] ?? item.source} · ${item.duration_min}m`;
  li.appendChild(badge);

  if (item.status === 'ignored') {
    const ig = document.createElement('span');
    ig.className = 'text-xs text-ink-dim';
    ig.textContent = 'ignored';
    li.appendChild(ig);
  }

  if (item.needs_review) {
    const flag = document.createElement('span');
    flag.className = 'text-xs text-danger';
    flag.textContent = 'review';
    li.appendChild(flag);
  }

  const amount = document.createElement('span');
  amount.className = 'text-sm tabular-nums text-ink w-20 text-right';
  amount.textContent = fmtMoney(item.amount_cents);
  li.appendChild(amount);

  li.appendChild(iconButton('Override', async () => {
    const entered = window.prompt(`Amount for ${item.client_name} on ${item.session_date}:`,
      (item.amount_cents / 100).toFixed(2));
    if (entered === null) return;
    const cents = parseMoney(entered);
    if (cents === null) { window.alert('Enter a number, e.g. 75 or 75.50'); return; }
    await put(`/api/v1/training_sessions/${item.id}`, {
      ...item, override_cents: cents, amount_cents: cents, rate_source: 'override', needs_review: false,
    });
    await loadSummary();
  }));

  li.appendChild(iconButton(item.status === 'ignored' ? 'Unignore' : 'Ignore', async () => {
    await put(`/api/v1/training_sessions/${item.id}`, {
      ...item, status: item.status === 'ignored' ? 'scheduled' : 'ignored',
    });
    await loadSummary();
  }));

  return li;
}

function renderSessions() {
  const s = state.summary;
  const sessions = s?.sessions ?? [];
  const body = panel(sessionsEl, 'Sessions',
    `${sessions.length} in ${monthLabel(state.month)}`);

  if (sessions.length === 0) {
    emptyLine(body, 'No sessions recorded for this month yet. Sync the calendar, or add one manually.');
    return;
  }

  let currentDate = null;
  let list = null;
  for (const item of sessions) {
    if (item.session_date !== currentDate) {
      currentDate = item.session_date;
      const h = document.createElement('h3');
      h.className = 'text-xs text-ink-dim pt-2';
      h.textContent = dayLabel(currentDate);
      body.appendChild(h);
      list = document.createElement('ul');
      body.appendChild(list);
    }
    list.appendChild(sessionRow(item));
  }
}

// ---- Sync ----
// A sync outcome arrives in two different shapes depending on where it comes
// from:
//   - POST /api/v1/calendar/sync returns calendar.Result: ok, events_seen,
//     created, updated, cancelled, failed, error, finished_at.
//   - GET /api/v1/finances/summary's `last_sync` is the persisted log row:
//     ok, events_seen, created_count, updated_count, cancelled_count, error,
//     finished_at — it never tracked a `failed` count.
// normalizeSyncResult folds both into one shape so a single renderer can
// handle either without the caller branching on where the data came from.
function normalizeSyncResult(raw) {
  return {
    ok: !!raw.ok,
    eventsSeen: raw.events_seen ?? 0,
    created: raw.created ?? raw.created_count ?? 0,
    updated: raw.updated ?? raw.updated_count ?? 0,
    cancelled: raw.cancelled ?? raw.cancelled_count ?? 0,
    failed: raw.failed ?? null, // only ever present on the immediate POST result
    error: raw.error || null,
    finishedAt: raw.finished_at ?? '',
  };
}

const SYNC_ALREADY_RUNNING = 'a sync is already running';

// Renders one sync outcome into the status line. A run that comes back
// ok:false but with events actually created/updated/cancelled means "some
// events stored, some did not" — a materially different situation from the
// feed being entirely unreachable, so it gets its own message and color
// rather than collapsing into a flat failure. Shared by the page-load/refetch
// path (last_sync) and the Sync button's own response.
function applySyncStatus(raw) {
  if (!raw) {
    syncStatusEl.className = 'text-xs text-ink-dim';
    syncStatusEl.textContent = 'Never synced.';
    return;
  }

  const r = normalizeSyncResult(raw);
  const when = r.finishedAt;

  if (r.ok) {
    syncStatusEl.className = 'text-xs text-ink-dim';
    syncStatusEl.textContent = `Last sync ${when} — ${r.eventsSeen} events.`;
    return;
  }

  if (r.error === SYNC_ALREADY_RUNNING) {
    // Not a failure — a concurrent run is already in flight. Say so plainly
    // instead of in the danger color, which would read as something broke.
    syncStatusEl.className = 'text-xs text-ink-dim';
    syncStatusEl.textContent = 'A sync is already running — try again shortly.';
    return;
  }

  const applied = r.created + r.updated + r.cancelled;
  if (applied > 0) {
    // Partial success: the run reported ok:false overall, but part of the
    // feed made it in before whatever went wrong.
    syncStatusEl.className = 'text-xs text-accent';
    const failedNote = r.failed !== null ? `, ${r.failed} failed` : '';
    syncStatusEl.textContent =
      `Sync partially applied ${when} — ${applied} events applied${failedNote}` +
      (r.error ? `: ${r.error}` : '.');
    return;
  }

  syncStatusEl.className = 'text-xs text-danger';
  syncStatusEl.textContent = `Last sync failed ${when}: ${r.error ?? 'unknown error'}`;
}

function renderSyncStatus() {
  applySyncStatus(state.summary?.last_sync ?? null);
}

syncBtn.addEventListener('click', async () => {
  syncBtn.disabled = true;
  syncBtn.textContent = 'Syncing…';
  const res = await post('/api/v1/calendar/sync', {});
  syncBtn.textContent = 'Sync now';
  syncBtn.disabled = false;
  await loadSummary();
  if (res.ok) {
    // The button's own response is strictly more detailed than the refetched
    // summary's last_sync (it alone carries `failed`), so it gets the final
    // say on the status line — applied after loadSummary's repaint above,
    // not instead of it, so the ledger/sessions still refresh either way.
    applySyncStatus(res.data);
  } else {
    syncStatusEl.className = 'text-xs text-danger';
    syncStatusEl.textContent = res.error ?? 'Sync request failed.';
  }
});

// ---- Loading ----
export async function loadSummary() {
  const res = await get(`/api/v1/finances/summary?month=${state.month}`);
  if (!res.ok) {
    state.summary = null;
    ledgerEl.replaceChildren();
    errorLine(ledgerEl, res.error ?? 'Failed to load finances.');
    return;
  }
  state.summary = res.data;
  renderAll();
}

// renderAll is the single repaint entry point. Tasks 14 and 15 add their panel
// renderers to this list; nothing else calls them.
export function renderAll() {
  monthLabelEl.textContent = monthLabel(state.month);
  renderLedger();
  renderSyncStatus();
  renderSessions();
  renderAllTime();
}

document.getElementById('month-prev').addEventListener('click', async () => {
  state.month = shiftMonth(state.month, -1);
  await loadSummary();
});
document.getElementById('month-next').addEventListener('click', async () => {
  state.month = shiftMonth(state.month, 1);
  await loadSummary();
});

// @inject-forms

async function init() {
  await loadSummary();
}
init();

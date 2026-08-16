import { get, post, put, del } from '/static/js/lib/api.js';
import { createModal, confirmAction } from '/static/js/lib/modal.js';

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
const today = new Date();
export const state = {
  // Local wall clock, not toISOString() -- that is UTC, and from ~8pm on the
  // last day of a month it would open the page on the next month.
  month: `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, '0')}`,
  summary: null,
  // Set when the summary fetch itself fails, so renderAll can still update
  // the month label and the panels backed by their own endpoints, instead of
  // leaving the previous month's data on screen mislabelled as the new one.
  summaryError: null,
  clients: [],
  rateRules: [],
  subscriptions: [],
};

// Last failure message per panel, keyed the same as the panel element names
// below. panel() rebuilds each section's DOM from scratch on every repaint,
// so a mutating action can't just append an error node to the live tree --
// it has to survive here and get re-shown by the panel's own render function.
const actionErrors = {
  sessions: null,
  shopping: null,
  subscriptions: null,
  clients: null,
  rates: null,
};

function clearActionErrors() {
  Object.keys(actionErrors).forEach((k) => { actionErrors[k] = null; });
}

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
export function panel(container, title, subtitle, action) {
  container.replaceChildren();

  const head = document.createElement('div');
  head.className = 'flex items-baseline justify-between gap-3';

  const h = document.createElement('h2');
  h.className = 'text-sm font-medium text-ink';
  h.textContent = title;
  head.appendChild(h);

  // subtitle and an optional header action (e.g. "+ Add item") share the
  // right-hand slot so the two-item justify-between split (title vs.
  // everything else) still holds with either, both, or neither present.
  const right = document.createElement('div');
  right.className = 'flex items-center gap-3';

  if (subtitle) {
    const s = document.createElement('span');
    s.className = 'text-xs text-ink-dim tabular-nums';
    s.textContent = subtitle;
    right.appendChild(s);
  }
  if (action) right.appendChild(action);

  if (right.childNodes.length > 0) head.appendChild(right);
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

// Small accent-outlined button for a panel's header row (e.g. "+ Add item"),
// styled to match bookmarks.js's "+ Add Bookmark" button.
export function headerActionButton(label, onClick) {
  const b = document.createElement('button');
  b.type = 'button';
  b.className =
    'px-3 py-1.5 text-xs border border-accent text-accent hover:bg-accent hover:text-canvas transition-colors';
  b.textContent = label;
  b.addEventListener('click', onClick);
  return b;
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

// Runs a single mutating call (put/post/del), records the outcome in
// actionErrors[panelKey] -- cleared on success, set to the server's message
// on failure or a network error -- then reloads so the panel repaints either
// way. Every row-level button below (override, ignore, bought, delete, rate
// changes, ...) goes through this so a 422/409/500 is never silently
// invisible.
async function mutate(panelKey, action, reload) {
  let res;
  try {
    res = await action();
  } catch (err) {
    actionErrors[panelKey] = 'Request failed -- check your connection and try again.';
    await reload();
    return { ok: false, error: actionErrors[panelKey] };
  }
  actionErrors[panelKey] = res.ok ? null : (res.error ?? 'Request failed.');
  await reload();
  return res;
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

  li.appendChild(iconButton('Override', (e) => openOverrideModal(item, e.currentTarget)));

  li.appendChild(iconButton(item.status === 'ignored' ? 'Unignore' : 'Ignore', async () => {
    await mutate('sessions', () => put(`/api/v1/training_sessions/${item.id}`, {
      ...item, status: item.status === 'ignored' ? 'scheduled' : 'ignored',
    }), loadSummary);
  }));

  return li;
}

function renderSessions() {
  const sessions = state.summary?.sessions ?? [];
  const subtitle = state.summaryError ? 'unavailable' : `${sessions.length} in ${monthLabel(state.month)}`;
  const body = panel(sessionsEl, 'Sessions', subtitle);

  if (state.summaryError) {
    errorLine(body, `Could not load sessions for ${monthLabel(state.month)}: ${state.summaryError}`);
    return;
  }

  if (actionErrors.sessions) errorLine(body, actionErrors.sessions);

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

// ---- Session override modal ----
let overrideItem = null;
const overrideModal = createModal('override-modal-title');
let overrideContextEl, overrideAmountInput, overrideSubmitBtn, overrideErrEl;

function buildOverrideModal() {
  const heading = document.createElement('h3');
  heading.id = 'override-modal-title';
  heading.className = 'text-sm font-semibold text-ink';
  heading.textContent = 'Override Amount';
  overrideModal.panel.appendChild(heading);

  const context = document.createElement('p');
  context.className = 'text-xs text-ink-dim';
  overrideModal.panel.appendChild(context);
  overrideContextEl = context;

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const amountLabel = document.createElement('label');
  amountLabel.className = 'block text-xs font-medium text-ink-dim mb-1';
  amountLabel.textContent = 'Amount';
  overrideAmountInput = document.createElement('input');
  overrideAmountInput.type = 'text';
  overrideAmountInput.name = 'amount';
  overrideAmountInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  overrideAmountInput.required = true;
  form.appendChild(amountLabel);
  form.appendChild(overrideAmountInput);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center justify-end gap-2';

  const cancelBtn = document.createElement('button');
  cancelBtn.type = 'button';
  cancelBtn.className =
    'px-4 py-2 border border-hairline text-ink-dim text-xs font-medium hover:text-ink hover:bg-surface-raised transition-colors';
  cancelBtn.textContent = 'Cancel';
  cancelBtn.addEventListener('click', () => overrideModal.close());
  btnRow.appendChild(cancelBtn);

  overrideSubmitBtn = document.createElement('button');
  overrideSubmitBtn.type = 'submit';
  overrideSubmitBtn.className =
    'px-4 py-2 border border-accent text-accent text-xs font-medium hover:bg-accent hover:text-canvas transition-colors';
  overrideSubmitBtn.textContent = 'Save';
  btnRow.appendChild(overrideSubmitBtn);

  form.appendChild(btnRow);

  overrideErrEl = document.createElement('p');
  overrideErrEl.className = 'text-sm text-danger mt-2 hidden';
  form.appendChild(overrideErrEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    overrideErrEl.classList.add('hidden');

    const cents = parseMoney(overrideAmountInput.value);
    if (cents === null) {
      overrideErrEl.textContent = 'Enter a number, e.g. 75 or 75.50';
      overrideErrEl.classList.remove('hidden');
      return;
    }

    overrideSubmitBtn.disabled = true;
    try {
      const res = await put(`/api/v1/training_sessions/${overrideItem.id}`, {
        ...overrideItem, override_cents: cents, amount_cents: cents, rate_source: 'override', needs_review: false,
      });
      if (res.ok) {
        overrideModal.close();
        await loadSummary();
      } else {
        overrideErrEl.textContent = res.error ?? 'Could not save the override.';
        overrideErrEl.classList.remove('hidden');
      }
    } catch (err) {
      overrideErrEl.textContent = 'Request failed — check your connection and try again.';
      overrideErrEl.classList.remove('hidden');
    } finally {
      overrideSubmitBtn.disabled = false;
    }
  });

  overrideModal.panel.appendChild(form);
}

function openOverrideModal(item, trigger) {
  overrideItem = item;
  overrideContextEl.textContent = `${item.client_name} — ${item.session_date}`;
  overrideAmountInput.value = (item.amount_cents / 100).toFixed(2);
  overrideErrEl.classList.add('hidden');
  overrideModal.open(trigger);
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
  try {
    const res = await post('/api/v1/calendar/sync', {});
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
  } catch (err) {
    // post() calls res.json() unguarded -- a network drop or a non-JSON
    // proxy response throws here rather than resolving to {ok:false}.
    syncStatusEl.className = 'text-xs text-danger';
    syncStatusEl.textContent = 'Sync request failed — check your connection and try again.';
  } finally {
    // Always restore the button, even if the request above threw, so it
    // never gets stuck disabled at "Syncing…".
    syncBtn.textContent = 'Sync now';
    syncBtn.disabled = false;
  }
});

// ---- Loading ----
export async function loadSummary() {
  const res = await get(`/api/v1/finances/summary?month=${state.month}`);
  if (!res.ok) {
    state.summary = null;
    state.summaryError = res.error ?? 'Failed to load finances.';
  } else {
    state.summary = res.data;
    state.summaryError = null;
  }
  renderAll();
}

// renderAll is the single repaint entry point. Tasks 14 and 15 add their panel
// renderers to this list; nothing else calls them.
//
// The month label and every panel backed by its own endpoint (Subscriptions,
// Clients, Rates) always repaint here, even when the summary fetch failed --
// only the ledger, Sessions and Shopping (all sourced from the summary
// payload) fall back to an error state. Without this, a failed summary on
// month navigation would leave the previous month's rows on screen under the
// new month's label.
export function renderAll() {
  monthLabelEl.textContent = monthLabel(state.month);

  if (state.summaryError) {
    ledgerEl.replaceChildren();
    errorLine(ledgerEl, state.summaryError);
  } else {
    renderLedger();
    renderSyncStatus();
  }

  renderSessions();
  renderShopping();
  renderSubscriptions();
  renderClients();
  renderRates();
  renderAllTime();
}

document.getElementById('month-prev').addEventListener('click', async () => {
  state.month = shiftMonth(state.month, -1);
  clearActionErrors();
  await loadSummary();
});
document.getElementById('month-next').addEventListener('click', async () => {
  state.month = shiftMonth(state.month, 1);
  clearActionErrors();
  await loadSummary();
});

// ---- Expense (Shopping) modal ----
const expenseModal = createModal('expense-modal-title');
let expNameInput, expAmountInput, expCategoryInput, expSubmitBtn, expErrEl;

function buildExpenseModal() {
  const heading = document.createElement('h3');
  heading.id = 'expense-modal-title';
  heading.className = 'text-sm font-semibold text-ink';
  heading.textContent = 'Add Item';
  expenseModal.panel.appendChild(heading);

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const nameLabel = document.createElement('label');
  nameLabel.className = 'block text-xs font-medium text-ink-dim mb-1';
  nameLabel.textContent = 'Item';
  expNameInput = document.createElement('input');
  expNameInput.type = 'text';
  expNameInput.name = 'name';
  expNameInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  expNameInput.required = true;
  form.appendChild(nameLabel);
  form.appendChild(expNameInput);

  const amountLabel = document.createElement('label');
  amountLabel.className = 'block text-xs font-medium text-ink-dim mb-1';
  amountLabel.textContent = 'Amount';
  expAmountInput = document.createElement('input');
  expAmountInput.type = 'text';
  expAmountInput.name = 'amount';
  expAmountInput.placeholder = '$0.00';
  expAmountInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  expAmountInput.required = true;
  form.appendChild(amountLabel);
  form.appendChild(expAmountInput);

  const categoryLabel = document.createElement('label');
  categoryLabel.className = 'block text-xs font-medium text-ink-dim mb-1';
  categoryLabel.textContent = 'Category (optional)';
  expCategoryInput = document.createElement('input');
  expCategoryInput.type = 'text';
  expCategoryInput.name = 'category';
  expCategoryInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  form.appendChild(categoryLabel);
  form.appendChild(expCategoryInput);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center justify-end gap-2';

  const cancelBtn = document.createElement('button');
  cancelBtn.type = 'button';
  cancelBtn.className =
    'px-4 py-2 border border-hairline text-ink-dim text-xs font-medium hover:text-ink hover:bg-surface-raised transition-colors';
  cancelBtn.textContent = 'Cancel';
  cancelBtn.addEventListener('click', () => expenseModal.close());
  btnRow.appendChild(cancelBtn);

  expSubmitBtn = document.createElement('button');
  expSubmitBtn.type = 'submit';
  expSubmitBtn.className =
    'px-4 py-2 border border-accent text-accent text-xs font-medium hover:bg-accent hover:text-canvas transition-colors';
  expSubmitBtn.textContent = 'Add Item';
  btnRow.appendChild(expSubmitBtn);

  form.appendChild(btnRow);

  expErrEl = document.createElement('p');
  expErrEl.className = 'text-sm text-danger mt-2 hidden';
  form.appendChild(expErrEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    expErrEl.classList.add('hidden');

    const cents = parseMoney(expAmountInput.value);
    if (!expNameInput.value.trim()) {
      expErrEl.textContent = 'Name is required.';
      expErrEl.classList.remove('hidden');
      return;
    }
    if (cents === null) {
      expErrEl.textContent = 'Enter an amount, e.g. 45 or 45.50';
      expErrEl.classList.remove('hidden');
      return;
    }

    expSubmitBtn.disabled = true;
    try {
      const res = await post('/api/v1/expenses', {
        name: expNameInput.value.trim(),
        amount_cents: cents,
        category: expCategoryInput.value.trim() || null,
        status: 'planned',
        incurred_on: `${state.month}-01`,
        notes: null,
      });
      if (res.ok) {
        expenseModal.close();
        await loadSummary();
      } else {
        expErrEl.textContent = res.error ?? 'Could not add the item.';
        expErrEl.classList.remove('hidden');
      }
    } catch (err) {
      expErrEl.textContent = 'Request failed — check your connection and try again.';
      expErrEl.classList.remove('hidden');
    } finally {
      expSubmitBtn.disabled = false;
    }
  });

  expenseModal.panel.appendChild(form);
}

function openExpenseModalForCreate(trigger) {
  expNameInput.value = '';
  expAmountInput.value = '';
  expCategoryInput.value = '';
  expErrEl.classList.add('hidden');
  expenseModal.open(trigger);
}

// ---- Shopping panel ----
const shoppingEl = document.getElementById('shopping-panel');

function expenseRow(item) {
  const li = document.createElement('li');
  li.className = 'flex flex-wrap items-center gap-x-3 gap-y-1 py-2 border-b border-hairline last:border-0';

  const name = document.createElement('span');
  name.className = 'text-sm text-ink flex-1 min-w-32';
  name.textContent = item.name;
  if (item.status === 'bought') name.classList.add('text-ink-dim', 'line-through');
  li.appendChild(name);

  if (item.category) {
    const cat = document.createElement('span');
    cat.className = 'text-xs text-ink-dim';
    cat.textContent = item.category;
    li.appendChild(cat);
  }

  const when = document.createElement('span');
  when.className = 'text-xs text-ink-dim tabular-nums';
  when.textContent = item.incurred_on;
  li.appendChild(when);

  const amount = document.createElement('span');
  amount.className = 'text-sm tabular-nums text-ink w-20 text-right';
  amount.textContent = fmtMoney(item.amount_cents);
  li.appendChild(amount);

  li.appendChild(iconButton(item.status === 'bought' ? 'Un-buy' : 'Bought', async () => {
    await mutate('shopping', () => put(`/api/v1/expenses/${item.id}`, {
      ...item,
      status: item.status === 'bought' ? 'planned' : 'bought',
      // Blank so the handler re-stamps it to today when marking bought.
      incurred_on: item.status === 'bought' ? item.incurred_on : '',
    }), loadSummary);
  }));

  li.appendChild(iconButton('Delete', async (e) => {
    const ok = await confirmAction({
      title: `Delete "${item.name}"?`,
      message: 'This cannot be undone.',
      confirmLabel: 'Delete',
      danger: true,
      trigger: e.currentTarget,
    });
    if (!ok) return;
    await mutate('shopping', () => del(`/api/v1/expenses/${item.id}`), loadSummary);
  }, true));

  return li;
}

function renderShopping() {
  const bought = state.summary?.spending.shopping_bought_cents ?? 0;
  const committed = state.summary?.spending.shopping_committed_cents ?? 0;
  const subtitle = state.summaryError ? 'unavailable' : `${fmtMoney(bought)} spent · ${fmtMoney(committed)} planned`;
  // The add-item modal doesn't depend on the summary fetch, so keep it
  // available even when the month's rows above couldn't load.
  const addBtn = headerActionButton('+ Add item', (e) => openExpenseModalForCreate(e.currentTarget));
  const body = panel(shoppingEl, 'Shopping', subtitle, addBtn);

  if (state.summaryError) {
    errorLine(body, `Could not load shopping for ${monthLabel(state.month)}: ${state.summaryError}`);
  } else {
    if (actionErrors.shopping) errorLine(body, actionErrors.shopping);
    const items = state.summary?.expenses ?? [];
    if (items.length === 0) {
      emptyLine(body, 'Nothing on the list this month.');
    } else {
      const list = document.createElement('ul');
      items.forEach((item) => list.appendChild(expenseRow(item)));
      body.appendChild(list);
    }
  }
}

// ---- Subscriptions panel ----
const subscriptionsEl = document.getElementById('subscriptions-panel');
const CADENCES = ['monthly', 'yearly', 'weekly'];

function subscriptionRow(item) {
  const li = document.createElement('li');
  li.className = 'flex flex-wrap items-center gap-x-3 gap-y-1 py-2 border-b border-hairline last:border-0';

  const name = document.createElement('span');
  name.className = 'text-sm text-ink flex-1 min-w-32';
  name.textContent = item.name;
  if (!item.is_active) name.classList.add('text-ink-dim', 'line-through');
  li.appendChild(name);

  const cadence = document.createElement('span');
  cadence.className = 'text-xs text-ink-dim';
  cadence.textContent = item.cadence;
  li.appendChild(cadence);

  const amount = document.createElement('span');
  amount.className = 'text-sm tabular-nums text-ink w-20 text-right';
  amount.textContent = fmtMoney(item.amount_cents);
  li.appendChild(amount);

  li.appendChild(iconButton(item.is_active ? 'Stop' : 'Resume', async () => {
    await mutate('subscriptions', () => put(`/api/v1/subscriptions/${item.id}`, { ...item, is_active: !item.is_active }), loadAll);
  }));

  li.appendChild(iconButton('Delete', async (e) => {
    const ok = await confirmAction({
      title: `Delete "${item.name}"?`,
      message: 'This cannot be undone.',
      confirmLabel: 'Delete',
      danger: true,
      trigger: e.currentTarget,
    });
    if (!ok) return;
    await mutate('subscriptions', () => del(`/api/v1/subscriptions/${item.id}`), loadAll);
  }, true));

  return li;
}

// ---- Subscription (add) modal ----
const subscriptionModal = createModal('subscription-modal-title');
let subNameInput, subAmountInput, subCadenceSelect, subSubmitBtn, subErrEl;

function buildSubscriptionModal() {
  const heading = document.createElement('h3');
  heading.id = 'subscription-modal-title';
  heading.className = 'text-sm font-semibold text-ink';
  heading.textContent = 'Add Subscription';
  subscriptionModal.panel.appendChild(heading);

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const nameLabel = document.createElement('label');
  nameLabel.className = 'block text-xs font-medium text-ink-dim mb-1';
  nameLabel.textContent = 'Name';
  subNameInput = document.createElement('input');
  subNameInput.type = 'text';
  subNameInput.name = 'name';
  subNameInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  subNameInput.required = true;
  form.appendChild(nameLabel);
  form.appendChild(subNameInput);

  const amountLabel = document.createElement('label');
  amountLabel.className = 'block text-xs font-medium text-ink-dim mb-1';
  amountLabel.textContent = 'Amount';
  subAmountInput = document.createElement('input');
  subAmountInput.type = 'text';
  subAmountInput.name = 'amount';
  subAmountInput.placeholder = '$0.00';
  subAmountInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  subAmountInput.required = true;
  form.appendChild(amountLabel);
  form.appendChild(subAmountInput);

  const cadenceLabel = document.createElement('label');
  cadenceLabel.className = 'block text-xs font-medium text-ink-dim mb-1';
  cadenceLabel.textContent = 'Cadence';
  subCadenceSelect = document.createElement('select');
  subCadenceSelect.name = 'cadence';
  subCadenceSelect.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink focus:outline-none focus:border-accent';
  CADENCES.forEach((c) => {
    const opt = document.createElement('option');
    opt.value = c;
    opt.textContent = c;
    subCadenceSelect.appendChild(opt);
  });
  form.appendChild(cadenceLabel);
  form.appendChild(subCadenceSelect);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center justify-end gap-2';

  const cancelBtn = document.createElement('button');
  cancelBtn.type = 'button';
  cancelBtn.className =
    'px-4 py-2 border border-hairline text-ink-dim text-xs font-medium hover:text-ink hover:bg-surface-raised transition-colors';
  cancelBtn.textContent = 'Cancel';
  cancelBtn.addEventListener('click', () => subscriptionModal.close());
  btnRow.appendChild(cancelBtn);

  subSubmitBtn = document.createElement('button');
  subSubmitBtn.type = 'submit';
  subSubmitBtn.className =
    'px-4 py-2 border border-accent text-accent text-xs font-medium hover:bg-accent hover:text-canvas transition-colors';
  subSubmitBtn.textContent = 'Add Subscription';
  btnRow.appendChild(subSubmitBtn);

  form.appendChild(btnRow);

  subErrEl = document.createElement('p');
  subErrEl.className = 'text-sm text-danger mt-2 hidden';
  form.appendChild(subErrEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    subErrEl.classList.add('hidden');

    const cents = parseMoney(subAmountInput.value);
    if (!subNameInput.value.trim() || cents === null) {
      subErrEl.textContent = 'Name and a numeric amount are both required.';
      subErrEl.classList.remove('hidden');
      return;
    }

    subSubmitBtn.disabled = true;
    try {
      const res = await post('/api/v1/subscriptions', {
        name: subNameInput.value.trim(),
        amount_cents: cents,
        cadence: subCadenceSelect.value,
        billing_day: null,
        is_active: true,
        started_on: '',
        ended_on: null,
        notes: null,
      });
      if (res.ok) {
        subscriptionModal.close();
        await loadAll();
      } else {
        subErrEl.textContent = res.error ?? 'Could not add the subscription.';
        subErrEl.classList.remove('hidden');
      }
    } catch (err) {
      subErrEl.textContent = 'Request failed — check your connection and try again.';
      subErrEl.classList.remove('hidden');
    } finally {
      subSubmitBtn.disabled = false;
    }
  });

  subscriptionModal.panel.appendChild(form);
}

function openSubscriptionModalForCreate(trigger) {
  subNameInput.value = '';
  subAmountInput.value = '';
  subCadenceSelect.value = CADENCES[0];
  subErrEl.classList.add('hidden');
  subscriptionModal.open(trigger);
}

async function loadSubscriptions() {
  const res = await get('/api/v1/subscriptions?limit=200&sort=name');
  state.subscriptions = res.ok ? (res.data ?? []) : [];
}

function renderSubscriptions() {
  // subscriptions_cents lives on the summary payload; when it failed to load,
  // fall back to the independently-loaded list rather than showing a false
  // $0.00 for "this month".
  const subtitle = state.summaryError
    ? `${state.subscriptions.length} recurring`
    : `${fmtMoney(state.summary?.spending.subscriptions_cents ?? 0)} this month`;
  const addBtn = headerActionButton('+ Add subscription', (e) => openSubscriptionModalForCreate(e.currentTarget));
  const body = panel(subscriptionsEl, 'Subscriptions', subtitle, addBtn);
  if (actionErrors.subscriptions) errorLine(body, actionErrors.subscriptions);

  if (state.subscriptions.length === 0) {
    emptyLine(body, 'No recurring payments recorded.');
  } else {
    const list = document.createElement('ul');
    state.subscriptions.forEach((item) => list.appendChild(subscriptionRow(item)));
    body.appendChild(list);
  }
}

// ---- Clients panel ----
const clientsEl = document.getElementById('clients-panel');

async function loadClients() {
  const res = await get('/api/v1/clients?limit=200&sort=name');
  state.clients = res.ok ? (res.data ?? []) : [];
}

async function createClient(name, rateCents, kind) {
  return post('/api/v1/clients', {
    name,
    match_name: name, // the calendar spelling is the match key
    email: null,
    phone: null,
    rate_cents: rateCents,
    kind,
    is_active: true,
    notes: null,
  });
}

function clientRow(item) {
  const li = document.createElement('li');
  li.className = 'flex flex-wrap items-center gap-x-3 gap-y-1 py-2 border-b border-hairline last:border-0';

  const name = document.createElement('span');
  name.className = 'text-sm text-ink flex-1 min-w-32';
  name.textContent = item.name;
  if (!item.is_active) name.classList.add('text-ink-dim', 'line-through');
  li.appendChild(name);

  // match_name is shown whenever it differs from the display name — that gap is
  // exactly where a mispriced session comes from.
  if (item.match_name !== item.name) {
    const match = document.createElement('span');
    match.className = 'text-xs text-ink-dim';
    match.textContent = `matches “${item.match_name}”`;
    li.appendChild(match);
  }

  const kind = document.createElement('span');
  kind.className = 'text-xs text-ink-dim';
  kind.textContent = item.kind;
  li.appendChild(kind);

  const rate = document.createElement('span');
  rate.className = 'text-sm tabular-nums text-ink w-20 text-right';
  rate.textContent = item.kind === 'ignored' ? '—' : fmtMoney(item.rate_cents);
  li.appendChild(rate);

  li.appendChild(iconButton('Rate', (e) => openClientRateModal(item, e.currentTarget)));

  li.appendChild(iconButton(item.kind === 'ignored' ? 'Un-ignore' : 'Ignore', async () => {
    await mutate('clients', () => put(`/api/v1/clients/${item.id}`, {
      ...item, kind: item.kind === 'ignored' ? 'independent' : 'ignored',
    }), loadAll);
  }));

  li.appendChild(iconButton('Delete', async (e) => {
    const ok = await confirmAction({
      title: `Delete "${item.name}"?`,
      message: 'This cannot be undone.',
      confirmLabel: 'Delete',
      danger: true,
      trigger: e.currentTarget,
    });
    if (!ok) return;
    await mutate('clients', () => del(`/api/v1/clients/${item.id}`), loadAll);
  }, true));

  return li;
}

// ---- Client rate (edit) modal ----
let rateEditItem = null;
const clientRateModal = createModal('client-rate-modal-title');
let clientRateHeadingEl, clientRateAmountInput, clientRateSubmitBtn, clientRateErrEl;

function buildClientRateModal() {
  const heading = document.createElement('h3');
  heading.id = 'client-rate-modal-title';
  heading.className = 'text-sm font-semibold text-ink';
  heading.textContent = 'Session Rate';
  clientRateModal.panel.appendChild(heading);
  clientRateHeadingEl = heading;

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const amountLabel = document.createElement('label');
  amountLabel.className = 'block text-xs font-medium text-ink-dim mb-1';
  amountLabel.textContent = 'Rate';
  clientRateAmountInput = document.createElement('input');
  clientRateAmountInput.type = 'text';
  clientRateAmountInput.name = 'rate';
  clientRateAmountInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  clientRateAmountInput.required = true;
  form.appendChild(amountLabel);
  form.appendChild(clientRateAmountInput);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center justify-end gap-2';

  const cancelBtn = document.createElement('button');
  cancelBtn.type = 'button';
  cancelBtn.className =
    'px-4 py-2 border border-hairline text-ink-dim text-xs font-medium hover:text-ink hover:bg-surface-raised transition-colors';
  cancelBtn.textContent = 'Cancel';
  cancelBtn.addEventListener('click', () => clientRateModal.close());
  btnRow.appendChild(cancelBtn);

  clientRateSubmitBtn = document.createElement('button');
  clientRateSubmitBtn.type = 'submit';
  clientRateSubmitBtn.className =
    'px-4 py-2 border border-accent text-accent text-xs font-medium hover:bg-accent hover:text-canvas transition-colors';
  clientRateSubmitBtn.textContent = 'Save';
  btnRow.appendChild(clientRateSubmitBtn);

  form.appendChild(btnRow);

  clientRateErrEl = document.createElement('p');
  clientRateErrEl.className = 'text-sm text-danger mt-2 hidden';
  form.appendChild(clientRateErrEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    clientRateErrEl.classList.add('hidden');

    const cents = parseMoney(clientRateAmountInput.value);
    if (cents === null) {
      clientRateErrEl.textContent = 'Enter a number, e.g. 100';
      clientRateErrEl.classList.remove('hidden');
      return;
    }

    clientRateSubmitBtn.disabled = true;
    try {
      // Only rate_cents changes here — every other field, including
      // match_name, is carried through unaltered from the row's own data.
      const res = await put(`/api/v1/clients/${rateEditItem.id}`, { ...rateEditItem, rate_cents: cents });
      if (res.ok) {
        clientRateModal.close();
        await loadAll();
      } else {
        clientRateErrEl.textContent = res.error ?? 'Could not save the rate.';
        clientRateErrEl.classList.remove('hidden');
      }
    } catch (err) {
      clientRateErrEl.textContent = 'Request failed — check your connection and try again.';
      clientRateErrEl.classList.remove('hidden');
    } finally {
      clientRateSubmitBtn.disabled = false;
    }
  });

  clientRateModal.panel.appendChild(form);
}

function openClientRateModal(item, trigger) {
  rateEditItem = item;
  clientRateHeadingEl.textContent = `Session Rate — ${item.name}`;
  clientRateAmountInput.value = (item.rate_cents / 100).toFixed(2);
  clientRateErrEl.classList.add('hidden');
  clientRateModal.open(trigger);
}

// The unmatched strip is the whole review workflow: a session the feed
// flagged for review because its name matched no client row, and two
// one-click ways to resolve it.
//
// That "matched no client row" set also includes a gym session whose
// duration matched no rate rule (needs_review=1, client_id IS NULL either
// way) — it isn't only missing independents. "Add at $100" is only correct
// for the latter; for a duration problem it creates a bogus $100 client
// instead of fixing the real cause, a missing rate rule.
function unmatchedStrip(container) {
  const names = state.summary?.unmatched_names ?? [];
  if (names.length === 0) return;

  const total = state.summary.needs_review_count ?? names.length;

  const box = document.createElement('div');
  box.className = 'border border-hairline bg-canvas p-3 space-y-2';

  const heading = document.createElement('p');
  heading.className = 'text-xs text-danger';
  heading.textContent = total > names.length
    ? `${names.length} of ${total} flagged sessions shown — names matching no client:`
    : `${names.length} flagged session(s) — names matching no client:`;
  box.appendChild(heading);

  const note = document.createElement('p');
  note.className = 'text-xs text-ink-dim';
  note.textContent =
    'A name lands here whenever its session matched no client row — including a gym ' +
    'session whose duration matched no rate rule. "Add at $100" is only correct if this ' +
    'really is an independent client; for a duration problem, fix Gym rates instead.';
  box.appendChild(note);

  names.forEach((name) => {
    const row = document.createElement('div');
    row.className = 'flex flex-wrap items-center gap-2';

    const label = document.createElement('span');
    label.className = 'text-sm text-ink flex-1 min-w-32';
    label.textContent = name; // textContent — this string came from the calendar
    row.appendChild(label);

    const addBtn = iconButton('Add at $100', async (e) => {
      await resolveUnmatched(e.currentTarget, () => createClient(name, 10000, 'independent'));
    });
    row.appendChild(addBtn);

    const ignoreBtn = iconButton('Always ignore', async (e) => {
      await resolveUnmatched(e.currentTarget, () => createClient(name, 0, 'ignored'));
    });
    row.appendChild(ignoreBtn);

    box.appendChild(row);
  });

  container.appendChild(box);
}

// Creating (or ignoring) a client from the review queue doesn't retroactively
// reprice existing sessions -- pricing only happens during a calendar sync.
// Without a sync here, loadAll() would repaint byte-identical data: the name
// stays in this strip, its sessions still show "review", and an ignored name
// keeps counting toward income until the background ticker's next run (up to
// 30 minutes). So: create the client, sync, then reload -- with an
// in-progress label on the button since the sync alone takes a few seconds.
async function resolveUnmatched(button, createClientFn) {
  button.disabled = true;
  button.textContent = 'Working…';
  try {
    const createRes = await createClientFn();
    if (!createRes.ok) {
      // e.g. 409 conflict on a duplicate match_name -- the client was not
      // created, so there is nothing to sync/reprice.
      actionErrors.clients = createRes.error ?? 'Could not save the client.';
      return;
    }

    button.textContent = 'Syncing…';
    const syncRes = await post('/api/v1/calendar/sync', {});
    if (!syncRes.ok) {
      actionErrors.clients = syncRes.error ?? 'Sync request failed; sessions may not be repriced yet.';
    } else if (syncRes.data?.ok === false) {
      // The client row was created regardless -- say so isn't needed since
      // reload below will already reflect the new client; just surface why
      // the sessions may still look unpriced.
      actionErrors.clients = syncRes.data.error ?? 'Sync did not complete cleanly; sessions may not be repriced yet.';
    } else {
      actionErrors.clients = null;
    }
  } catch (err) {
    actionErrors.clients = 'Request failed — check your connection and try again.';
  } finally {
    // loadAll() repaints the whole panel (and replaces this button), so
    // there's no separate re-enable step needed here.
    await loadAll();
  }
}

// ---- Client (add) modal ----
const clientAddModal = createModal('client-add-modal-title');
let clientAddNameInput, clientAddRateInput, clientAddSubmitBtn, clientAddErrEl;

function buildClientAddModal() {
  const heading = document.createElement('h3');
  heading.id = 'client-add-modal-title';
  heading.className = 'text-sm font-semibold text-ink';
  heading.textContent = 'Add Client';
  clientAddModal.panel.appendChild(heading);

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const nameLabel = document.createElement('label');
  nameLabel.className = 'block text-xs font-medium text-ink-dim mb-1';
  nameLabel.textContent = 'Name exactly as it appears in the calendar';
  clientAddNameInput = document.createElement('input');
  clientAddNameInput.type = 'text';
  clientAddNameInput.name = 'name';
  clientAddNameInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  clientAddNameInput.required = true;
  form.appendChild(nameLabel);
  form.appendChild(clientAddNameInput);

  const rateLabel = document.createElement('label');
  rateLabel.className = 'block text-xs font-medium text-ink-dim mb-1';
  rateLabel.textContent = 'Rate';
  clientAddRateInput = document.createElement('input');
  clientAddRateInput.type = 'text';
  clientAddRateInput.name = 'rate';
  clientAddRateInput.placeholder = '$100.00';
  clientAddRateInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  form.appendChild(rateLabel);
  form.appendChild(clientAddRateInput);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center justify-end gap-2';

  const cancelBtn = document.createElement('button');
  cancelBtn.type = 'button';
  cancelBtn.className =
    'px-4 py-2 border border-hairline text-ink-dim text-xs font-medium hover:text-ink hover:bg-surface-raised transition-colors';
  cancelBtn.textContent = 'Cancel';
  cancelBtn.addEventListener('click', () => clientAddModal.close());
  btnRow.appendChild(cancelBtn);

  clientAddSubmitBtn = document.createElement('button');
  clientAddSubmitBtn.type = 'submit';
  clientAddSubmitBtn.className =
    'px-4 py-2 border border-accent text-accent text-xs font-medium hover:bg-accent hover:text-canvas transition-colors';
  clientAddSubmitBtn.textContent = 'Add Client';
  btnRow.appendChild(clientAddSubmitBtn);

  form.appendChild(btnRow);

  clientAddErrEl = document.createElement('p');
  clientAddErrEl.className = 'text-sm text-danger mt-2 hidden';
  form.appendChild(clientAddErrEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    clientAddErrEl.classList.add('hidden');

    const name = clientAddNameInput.value.trim();
    const cents = clientAddRateInput.value.trim() === '' ? 10000 : parseMoney(clientAddRateInput.value);
    if (!name || cents === null) {
      clientAddErrEl.textContent = 'A name is required, and the rate must be a number.';
      clientAddErrEl.classList.remove('hidden');
      return;
    }

    clientAddSubmitBtn.disabled = true;
    try {
      // createClient sets match_name to this exact spelling -- the calendar
      // matcher is exact, so it must never be trimmed or altered further.
      const res = await createClient(name, cents, 'independent');
      if (res.ok) {
        clientAddModal.close();
        await loadAll();
      } else {
        clientAddErrEl.textContent = res.error ?? 'Could not add the client.';
        clientAddErrEl.classList.remove('hidden');
      }
    } catch (err) {
      clientAddErrEl.textContent = 'Request failed — check your connection and try again.';
      clientAddErrEl.classList.remove('hidden');
    } finally {
      clientAddSubmitBtn.disabled = false;
    }
  });

  clientAddModal.panel.appendChild(form);
}

function openClientAddModalForCreate(trigger) {
  clientAddNameInput.value = '';
  clientAddRateInput.value = '';
  clientAddErrEl.classList.add('hidden');
  clientAddModal.open(trigger);
}

function renderClients() {
  const independents = state.clients.filter((c) => c.kind !== 'ignored').length;
  const addBtn = headerActionButton('+ Add client', (e) => openClientAddModalForCreate(e.currentTarget));
  const body = panel(clientsEl, 'Clients', `${independents} independent`, addBtn);
  if (actionErrors.clients) errorLine(body, actionErrors.clients);

  if (state.summaryError) {
    emptyLine(body, 'Flagged-session review is unavailable until this month’s data reloads.');
  } else {
    unmatchedStrip(body);
  }

  if (state.clients.length === 0) {
    emptyLine(body, 'No clients yet.');
  } else {
    const list = document.createElement('ul');
    state.clients.forEach((item) => list.appendChild(clientRow(item)));
    body.appendChild(list);
  }
}

// ---- Rate rules panel ----
const ratesEl = document.getElementById('rates-panel');

async function loadRateRules() {
  const res = await get('/api/v1/rate_rules?limit=50&sort=duration_min');
  state.rateRules = res.ok ? (res.data ?? []) : [];
}

function rateRow(item) {
  const li = document.createElement('li');
  li.className = 'flex items-center gap-3 py-2 border-b border-hairline last:border-0';

  const label = document.createElement('span');
  label.className = 'text-sm text-ink flex-1';
  label.textContent = item.label || `${item.duration_min} minutes`;
  li.appendChild(label);

  const amount = document.createElement('span');
  amount.className = 'text-sm tabular-nums text-ink w-20 text-right';
  amount.textContent = fmtMoney(item.amount_cents);
  li.appendChild(amount);

  li.appendChild(iconButton('Edit', (e) => openRateRuleModal(item, e.currentTarget)));

  return li;
}

// ---- Rate rule (edit) modal ----
let rateRuleEditItem = null;
const rateRuleModal = createModal('rate-rule-modal-title');
let rateRuleHeadingEl, rateRuleAmountInput, rateRuleSubmitBtn, rateRuleErrEl;

function buildRateRuleModal() {
  const heading = document.createElement('h3');
  heading.id = 'rate-rule-modal-title';
  heading.className = 'text-sm font-semibold text-ink';
  heading.textContent = 'Edit Rate';
  rateRuleModal.panel.appendChild(heading);
  rateRuleHeadingEl = heading;

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const amountLabel = document.createElement('label');
  amountLabel.className = 'block text-xs font-medium text-ink-dim mb-1';
  amountLabel.textContent = 'Amount';
  rateRuleAmountInput = document.createElement('input');
  rateRuleAmountInput.type = 'text';
  rateRuleAmountInput.name = 'amount';
  rateRuleAmountInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  rateRuleAmountInput.required = true;
  form.appendChild(amountLabel);
  form.appendChild(rateRuleAmountInput);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center justify-end gap-2';

  const cancelBtn = document.createElement('button');
  cancelBtn.type = 'button';
  cancelBtn.className =
    'px-4 py-2 border border-hairline text-ink-dim text-xs font-medium hover:text-ink hover:bg-surface-raised transition-colors';
  cancelBtn.textContent = 'Cancel';
  cancelBtn.addEventListener('click', () => rateRuleModal.close());
  btnRow.appendChild(cancelBtn);

  rateRuleSubmitBtn = document.createElement('button');
  rateRuleSubmitBtn.type = 'submit';
  rateRuleSubmitBtn.className =
    'px-4 py-2 border border-accent text-accent text-xs font-medium hover:bg-accent hover:text-canvas transition-colors';
  rateRuleSubmitBtn.textContent = 'Save';
  btnRow.appendChild(rateRuleSubmitBtn);

  form.appendChild(btnRow);

  rateRuleErrEl = document.createElement('p');
  rateRuleErrEl.className = 'text-sm text-danger mt-2 hidden';
  form.appendChild(rateRuleErrEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    rateRuleErrEl.classList.add('hidden');

    const cents = parseMoney(rateRuleAmountInput.value);
    if (cents === null) {
      rateRuleErrEl.textContent = 'Enter a number, e.g. 50';
      rateRuleErrEl.classList.remove('hidden');
      return;
    }

    rateRuleSubmitBtn.disabled = true;
    try {
      const res = await put(`/api/v1/rate_rules/${rateRuleEditItem.id}`, { ...rateRuleEditItem, amount_cents: cents });
      if (res.ok) {
        rateRuleModal.close();
        await loadAll();
      } else {
        rateRuleErrEl.textContent = res.error ?? 'Could not save the rate.';
        rateRuleErrEl.classList.remove('hidden');
      }
    } catch (err) {
      rateRuleErrEl.textContent = 'Request failed — check your connection and try again.';
      rateRuleErrEl.classList.remove('hidden');
    } finally {
      rateRuleSubmitBtn.disabled = false;
    }
  });

  rateRuleModal.panel.appendChild(form);
}

function openRateRuleModal(item, trigger) {
  rateRuleEditItem = item;
  rateRuleHeadingEl.textContent = `Edit Rate — ${item.label || item.duration_min + ' minutes'}`;
  rateRuleAmountInput.value = (item.amount_cents / 100).toFixed(2);
  rateRuleErrEl.classList.add('hidden');
  rateRuleModal.open(trigger);
}

function renderRates() {
  const body = panel(ratesEl, 'Gym rates', 'by session length');
  if (actionErrors.rates) errorLine(body, actionErrors.rates);

  if (state.rateRules.length === 0) {
    emptyLine(body, 'No duration rules — gym sessions will be flagged for review.');
    return;
  }
  const list = document.createElement('ul');
  state.rateRules.forEach((item) => list.appendChild(rateRow(item)));
  body.appendChild(list);

  const note = document.createElement('p');
  note.className = 'text-xs text-ink-dim pt-1';
  note.textContent =
    'These apply to gym sessions only. Independent clients are priced from the Clients list above.';
  body.appendChild(note);
}

// @inject-forms

buildOverrideModal();
buildExpenseModal();
buildSubscriptionModal();
buildClientAddModal();
buildClientRateModal();
buildRateRuleModal();

// loadAll refetches everything the page owns. The summary carries the month's
// sessions and shopping rows; subscriptions come from their own list endpoint
// because they are not month-scoped.
export async function loadAll() {
  await Promise.all([loadSubscriptions(), loadClients(), loadRateRules()]);
  await loadSummary();
}

async function init() {
  await loadAll();
}
init();

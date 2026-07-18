# Dark Mode Toggle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a persistent light/dark theme toggle across all 7 static pages of the Homelab app.

**Architecture:** All page colors already route through CSS custom properties defined once in `input.css` (`--color-canvas`, `--color-ink`, etc.). Dark mode is a single override block for those properties, gated by `[data-theme="dark"]` on `<html>`. A new shared module `static/js/lib/theme.js` sets that attribute (from `localStorage` or `prefers-color-scheme`), wires click handlers on toggle buttons, and persists explicit choices. A tiny inline script in each page's `<head>` sets the attribute before first paint to avoid a flash of the wrong theme. No Go/backend/model changes — pure static assets.

**Tech Stack:** Tailwind CSS v4 (`@theme` custom properties), vanilla JS ES modules, `localStorage`, `matchMedia`.

## Global Constraints

- No raw SQL, no HTML rendering in Go handlers, no new backend routes — this feature touches only `static/css`, `static/js/lib`, and `static/pages`.
- JS safety: never `innerHTML` on user data (n/a here — no user data involved), never `eval`/`new Function`, use `textContent`/`classList`/`setAttribute` only.
- No Node.js/NPM — Tailwind CLI standalone recompiles CSS automatically on `docker compose restart app`; no build step to run manually.
- `static/js/lib/*.js` is hand-written infrastructure per this project's `CLAUDE.md` — no MCP scaffold tool applies to this feature (confirmed during brainstorming: no new model, page, or handler is created).
- Spec: `docs/superpowers/specs/2026-07-17-dark-mode-toggle-design.md`.

---

### Task 1: Dark palette CSS override

**Files:**
- Modify: `src/app/static/css/input.css`

**Interfaces:**
- Produces: CSS custom properties `--color-canvas`, `--color-surface`, `--color-surface-raised`, `--color-hairline`, `--color-ink`, `--color-ink-dim`, `--color-accent`, `--color-accent-dim`, `--color-ok`, `--color-danger` re-defined under `:root[data-theme="dark"]`. Every existing Tailwind utility class (`bg-canvas`, `text-ink`, etc.) on every page consumes these automatically — no other file needs to change for colors to flip.

- [ ] **Step 1: Add the dark override block**

In `src/app/static/css/input.css`, insert immediately after the closing `}` of the existing `@theme { ... }` block (i.e. right before the `@layer base {` block):

```css
:root[data-theme="dark"] {
  --color-canvas: #1c1a16;
  --color-surface: #262319;
  --color-surface-raised: #322d21;
  --color-hairline: #3d382a;
  --color-ink: #f0ece2;
  --color-ink-dim: #a89d87;
  --color-accent: #c98a54;
  --color-accent-dim: #a8632b;
  --color-ok: #6fa87f;
  --color-danger: #d16b60;
}
```

- [ ] **Step 2: Rebuild and verify the override compiled**

Run:
```bash
docker compose restart app
sleep 2
grep -c '\[data-theme="dark"\]' src/app/static/css/style.css
```
Expected: a number greater than `0` (Tailwind's standalone CLI passes the raw CSS rule through unchanged since it targets no Tailwind utility class, just a plain override block).

- [ ] **Step 3: Commit**

```bash
git add src/app/static/css/input.css
git commit -m "feat: add dark theme CSS token overrides"
```

---

### Task 2: Shared theme.js module

**Files:**
- Create: `src/app/static/js/lib/theme.js`

**Interfaces:**
- Consumes: `document.documentElement` (sets `dataset.theme`), any DOM elements matching `[data-theme-toggle]` each expected to contain a child matching `.theme-icon-sun` and a child matching `.theme-icon-moon` (added to page markup in Task 3/4).
- Produces: side-effecting module — importing it (`<script type="module" src="/static/js/lib/theme.js">`) immediately runs `initTheme()`, which applies the effective theme and wires click listeners. No exported functions are consumed elsewhere; this file is self-contained.

- [ ] **Step 1: Write the module**

Create `src/app/static/js/lib/theme.js`:

```js
const STORAGE_KEY = 'theme';

function getStoredTheme() {
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch (e) {
    return null;
  }
}

function getEffectiveTheme() {
  const stored = getStoredTheme();
  if (stored === 'light' || stored === 'dark') return stored;
  return matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function applyTheme(theme) {
  document.documentElement.dataset.theme = theme;
  document.querySelectorAll('[data-theme-toggle]').forEach((btn) => {
    const sun = btn.querySelector('.theme-icon-sun');
    const moon = btn.querySelector('.theme-icon-moon');
    const isDark = theme === 'dark';
    if (sun) sun.classList.toggle('hidden', !isDark);
    if (moon) moon.classList.toggle('hidden', isDark);
    btn.setAttribute('aria-pressed', String(isDark));
  });
}

function toggleTheme() {
  const current = document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light';
  const next = current === 'dark' ? 'light' : 'dark';
  applyTheme(next);
  try {
    localStorage.setItem(STORAGE_KEY, next);
  } catch (e) {
    // storage unavailable (e.g. private browsing) — theme still applies for this session
  }
}

export function initTheme() {
  applyTheme(getEffectiveTheme());
  document.querySelectorAll('[data-theme-toggle]').forEach((btn) => {
    btn.addEventListener('click', toggleTheme);
  });
}

initTheme();
```

- [ ] **Step 2: Verify syntax**

No Node/npm in this project (per Global Constraints), so verify via the Go toolchain's static file serving instead:
```bash
docker compose restart app
sleep 2
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:1234/static/js/lib/theme.js
```
Expected: `200`

- [ ] **Step 3: Commit**

```bash
git add src/app/static/js/lib/theme.js
git commit -m "feat: add shared theme.js module for dark mode toggle"
```

---

### Task 3: Wire dark mode into home.html (end-to-end proof)

**Files:**
- Modify: `src/app/static/pages/home.html`

**Interfaces:**
- Consumes: `theme.js` (Task 2) via `<script type="module" src="/static/js/lib/theme.js">`; dark CSS override (Task 1).
- Produces: the exact markup pattern (head snippet, desktop button, mobile button, script tag) that Task 4 replicates verbatim into the remaining 6 pages.

- [ ] **Step 1: Add the anti-flash inline script to `<head>`**

In `src/app/static/pages/home.html`, replace:

```html
  <title>Dashboard · Homelab</title>
  <link rel="stylesheet" href="/static/css/style.css">
```

with:

```html
  <title>Dashboard · Homelab</title>
  <script>
    (function() {
      try {
        var t = localStorage.getItem('theme');
        if (!t) t = matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
        document.documentElement.dataset.theme = t;
      } catch (e) {}
    })();
  </script>
  <link rel="stylesheet" href="/static/css/style.css">
```

- [ ] **Step 2: Add the desktop toggle button**

Replace:

```html
      <div class="hidden md:flex items-center gap-1.5 shrink-0 text-xs text-ink-dim tabular-nums">
        <span>LOCAL</span>
        <time id="clock">--:--:--</time>
      </div>
```

with:

```html
      <div class="hidden md:flex items-center gap-3 shrink-0 text-xs text-ink-dim tabular-nums">
        <span class="flex items-center gap-1.5">
          <span>LOCAL</span>
          <time id="clock">--:--:--</time>
        </span>
        <button type="button" data-theme-toggle aria-pressed="false" aria-label="Toggle dark mode" class="inline-flex items-center justify-center h-7 w-7 text-ink-dim hover:text-ink transition-colors">
          <svg class="theme-icon-sun w-4 h-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
            <path d="M10 15a5 5 0 100-10 5 5 0 000 10zM10 0a1 1 0 011 1v1a1 1 0 11-2 0V1a1 1 0 011-1zm0 16a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM3.05 3.05a1 1 0 011.414 0l.707.707a1 1 0 11-1.414 1.414l-.707-.707a1 1 0 010-1.414zm11.78 11.78a1 1 0 011.415 0l.707.707a1 1 0 01-1.415 1.415l-.707-.707a1 1 0 010-1.415zM0 10a1 1 0 011-1h1a1 1 0 110 2H1a1 1 0 01-1-1zm16 0a1 1 0 011-1h1a1 1 0 110 2h-1a1 1 0 01-1-1zM3.05 16.95a1 1 0 010-1.414l.707-.707a1 1 0 111.414 1.414l-.707.707a1 1 0 01-1.414 0zm11.78-11.78a1 1 0 010-1.414l.707-.707a1 1 0 111.415 1.414l-.707.707a1 1 0 01-1.415 0z"/>
          </svg>
          <svg class="theme-icon-moon w-4 h-4 hidden" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
            <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z"/>
          </svg>
        </button>
      </div>
```

(Icon convention: moon visible in light mode = "click to go dark"; sun visible in dark mode = "click to go light". `theme.js`'s `applyTheme` toggles the `hidden` class on these two children.)

- [ ] **Step 3: Add the mobile drawer toggle button**

Replace:

```html
    <div class="mt-auto px-4 py-3 border-t border-hairline text-xs text-ink-dim flex items-center gap-1.5 tabular-nums shrink-0">
      <span>LOCAL</span>
      <time id="clock-mobile">--:--:--</time>
    </div>
```

with:

```html
    <div class="mt-auto px-4 py-3 border-t border-hairline text-xs text-ink-dim flex items-center justify-between gap-1.5 tabular-nums shrink-0">
      <span class="flex items-center gap-1.5">
        <span>LOCAL</span>
        <time id="clock-mobile">--:--:--</time>
      </span>
      <button type="button" data-theme-toggle aria-pressed="false" aria-label="Toggle dark mode" class="inline-flex items-center justify-center h-7 w-7 text-ink-dim hover:text-ink transition-colors">
        <svg class="theme-icon-sun w-4 h-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
          <path d="M10 15a5 5 0 100-10 5 5 0 000 10zM10 0a1 1 0 011 1v1a1 1 0 11-2 0V1a1 1 0 011-1zm0 16a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM3.05 3.05a1 1 0 011.414 0l.707.707a1 1 0 11-1.414 1.414l-.707-.707a1 1 0 010-1.414zm11.78 11.78a1 1 0 011.415 0l.707.707a1 1 0 01-1.415 1.415l-.707-.707a1 1 0 010-1.415zM0 10a1 1 0 011-1h1a1 1 0 110 2H1a1 1 0 01-1-1zm16 0a1 1 0 011-1h1a1 1 0 110 2h-1a1 1 0 01-1-1zM3.05 16.95a1 1 0 010-1.414l.707-.707a1 1 0 111.414 1.414l-.707.707a1 1 0 01-1.414 0zm11.78-11.78a1 1 0 010-1.414l.707-.707a1 1 0 111.415 1.414l-.707.707a1 1 0 01-1.415 0z"/>
        </svg>
        <svg class="theme-icon-moon w-4 h-4 hidden" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
          <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z"/>
        </svg>
      </button>
    </div>
```

- [ ] **Step 4: Load the theme module**

Replace:

```html
  <script type="module" src="/static/js/home.js"></script>
```

with:

```html
  <script type="module" src="/static/js/lib/theme.js"></script>
  <script type="module" src="/static/js/home.js"></script>
```

- [ ] **Step 5: Restart and verify markup served**

```bash
docker compose restart app
sleep 2
curl -s http://localhost:1234/static/pages/home.html | grep -c 'data-theme-toggle'
```
Expected: `2` (desktop + mobile button).

- [ ] **Step 6: Manual browser verification**

Open `http://localhost:1234/static/pages/home.html`:
1. Click the theme toggle in the header — canvas/surface/border/text colors should flip to the dark palette immediately, icon should swap sun/moon.
2. Reload the page — dark mode should persist (no flash of light mode first).
3. Open devtools → rendering → emulate `prefers-color-scheme: dark`, clear `localStorage` (`localStorage.removeItem('theme')` in console), reload — page should open in dark mode with no flash.
4. Tab to the toggle button and press Enter/Space — confirm it's keyboard-operable and the focus ring (`:focus-visible`) is legible in both themes.

- [ ] **Step 7: Commit**

```bash
git add src/app/static/pages/home.html
git commit -m "feat: wire dark mode toggle into home page"
```

---

### Task 4: Replicate toggle to remaining 6 pages

**Files:**
- Modify: `src/app/static/pages/reminders.html`
- Modify: `src/app/static/pages/todos.html`
- Modify: `src/app/static/pages/journal.html`
- Modify: `src/app/static/pages/codex.html`
- Modify: `src/app/static/pages/bookmarks.html`
- Modify: `src/app/static/pages/logger.html`

**Interfaces:**
- Consumes: identical markup pattern proven in Task 3 (head snippet, desktop button, mobile button, `theme.js` script tag). All 7 pages share byte-identical header/drawer markup (confirmed during brainstorming), so the same 4 edits apply to each file — only each file's own `<title>` line and existing per-page `<script type="module" src="/static/js/<page>.js">` line differ, and those are left untouched.

For **each** of the 6 files listed above, apply the same 4 edits used in Task 3:

- [ ] **Step 1: Add the anti-flash inline script to `<head>`**

Insert immediately before the existing `<link rel="stylesheet" href="/static/css/style.css">` line in each file:

```html
  <script>
    (function() {
      try {
        var t = localStorage.getItem('theme');
        if (!t) t = matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
        document.documentElement.dataset.theme = t;
      } catch (e) {}
    })();
  </script>
```

- [ ] **Step 2: Add the desktop toggle button**

In each file, replace:

```html
      <div class="hidden md:flex items-center gap-1.5 shrink-0 text-xs text-ink-dim tabular-nums">
        <span>LOCAL</span>
        <time id="clock">--:--:--</time>
      </div>
```

with the same block used in Task 3 Step 2 (identical across all files — `<time id="clock">` is the same id in every page):

```html
      <div class="hidden md:flex items-center gap-3 shrink-0 text-xs text-ink-dim tabular-nums">
        <span class="flex items-center gap-1.5">
          <span>LOCAL</span>
          <time id="clock">--:--:--</time>
        </span>
        <button type="button" data-theme-toggle aria-pressed="false" aria-label="Toggle dark mode" class="inline-flex items-center justify-center h-7 w-7 text-ink-dim hover:text-ink transition-colors">
          <svg class="theme-icon-sun w-4 h-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
            <path d="M10 15a5 5 0 100-10 5 5 0 000 10zM10 0a1 1 0 011 1v1a1 1 0 11-2 0V1a1 1 0 011-1zm0 16a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM3.05 3.05a1 1 0 011.414 0l.707.707a1 1 0 11-1.414 1.414l-.707-.707a1 1 0 010-1.414zm11.78 11.78a1 1 0 011.415 0l.707.707a1 1 0 01-1.415 1.415l-.707-.707a1 1 0 010-1.415zM0 10a1 1 0 011-1h1a1 1 0 110 2H1a1 1 0 01-1-1zm16 0a1 1 0 011-1h1a1 1 0 110 2h-1a1 1 0 01-1-1zM3.05 16.95a1 1 0 010-1.414l.707-.707a1 1 0 111.414 1.414l-.707.707a1 1 0 01-1.414 0zm11.78-11.78a1 1 0 010-1.414l.707-.707a1 1 0 111.415 1.414l-.707.707a1 1 0 01-1.415 0z"/>
          </svg>
          <svg class="theme-icon-moon w-4 h-4 hidden" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
            <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z"/>
          </svg>
        </button>
      </div>
```

- [ ] **Step 3: Add the mobile drawer toggle button**

In each file, replace:

```html
    <div class="mt-auto px-4 py-3 border-t border-hairline text-xs text-ink-dim flex items-center gap-1.5 tabular-nums shrink-0">
      <span>LOCAL</span>
      <time id="clock-mobile">--:--:--</time>
    </div>
```

with the same block used in Task 3 Step 3:

```html
    <div class="mt-auto px-4 py-3 border-t border-hairline text-xs text-ink-dim flex items-center justify-between gap-1.5 tabular-nums shrink-0">
      <span class="flex items-center gap-1.5">
        <span>LOCAL</span>
        <time id="clock-mobile">--:--:--</time>
      </span>
      <button type="button" data-theme-toggle aria-pressed="false" aria-label="Toggle dark mode" class="inline-flex items-center justify-center h-7 w-7 text-ink-dim hover:text-ink transition-colors">
        <svg class="theme-icon-sun w-4 h-4" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
          <path d="M10 15a5 5 0 100-10 5 5 0 000 10zM10 0a1 1 0 011 1v1a1 1 0 11-2 0V1a1 1 0 011-1zm0 16a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM3.05 3.05a1 1 0 011.414 0l.707.707a1 1 0 11-1.414 1.414l-.707-.707a1 1 0 010-1.414zm11.78 11.78a1 1 0 011.415 0l.707.707a1 1 0 01-1.415 1.415l-.707-.707a1 1 0 010-1.415zM0 10a1 1 0 011-1h1a1 1 0 110 2H1a1 1 0 01-1-1zm16 0a1 1 0 011-1h1a1 1 0 110 2h-1a1 1 0 01-1-1zM3.05 16.95a1 1 0 010-1.414l.707-.707a1 1 0 111.414 1.414l-.707.707a1 1 0 01-1.414 0zm11.78-11.78a1 1 0 010-1.414l.707-.707a1 1 0 111.415 1.414l-.707.707a1 1 0 01-1.415 0z"/>
        </svg>
        <svg class="theme-icon-moon w-4 h-4 hidden" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
          <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z"/>
        </svg>
      </button>
    </div>
```

- [ ] **Step 4: Load the theme module**

In each file, insert immediately before its existing per-page module script tag, e.g. for `reminders.html`:

```html
  <script type="module" src="/static/js/lib/theme.js"></script>
  <script type="module" src="/static/js/reminders.js"></script>
```

(Same pattern for `todos.js`, `journal.js`, `codex.js`, `bookmarks.js`, `logger.js` respectively — each file gets the `theme.js` line added directly above its own existing script tag.)

- [ ] **Step 5: Restart and verify all 7 pages serve the toggle**

```bash
docker compose restart app
sleep 2
for p in home reminders todos journal codex bookmarks logger; do
  echo -n "$p: "
  curl -s http://localhost:1234/static/pages/$p.html | grep -c 'data-theme-toggle'
done
```
Expected: `2` for every page.

- [ ] **Step 6: Manual cross-page verification**

1. Open `home.html`, click the toggle to switch to dark.
2. Navigate to each of the other 6 pages via the nav bar — confirm dark mode is still active on every one (proves `localStorage` persistence works across pages, not just reload).
3. Toggle back to light on any page, navigate around again — confirm light mode persists everywhere.
4. Resize to mobile width, open the hamburger drawer on 2-3 pages, confirm the drawer's toggle button is present and functional.

- [ ] **Step 7: Commit**

```bash
git add src/app/static/pages/reminders.html src/app/static/pages/todos.html src/app/static/pages/journal.html src/app/static/pages/codex.html src/app/static/pages/bookmarks.html src/app/static/pages/logger.html
git commit -m "feat: extend dark mode toggle to remaining pages"
```

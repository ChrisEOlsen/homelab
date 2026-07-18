# Dark Mode Toggle — Design

## Goal
Add a light/dark theme toggle across all 7 static pages, purely for styling — no backend, no new models, no new routes.

## Context
Colors are already centralized as CSS custom properties in `src/app/static/css/input.css` under `@theme` (`--color-canvas`, `--color-surface`, `--color-surface-raised`, `--color-hairline`, `--color-ink`, `--color-ink-dim`, `--color-accent`, `--color-accent-dim`, `--color-ok`, `--color-danger`). All 7 pages (`home`, `reminders`, `todos`, `journal`, `codex`, `bookmarks`, `logger`) consume these via Tailwind utility classes (`bg-canvas`, `text-ink`, etc.) — no page uses raw hex or `dark:` variants. Header/nav markup (desktop bar + mobile drawer) is duplicated verbatim across all 7 `.html` files; the per-page nav-toggle/clock wiring is duplicated across all 7 `.js` files. This project's `CLAUDE.md` treats `static/js/lib/*.js` as manually-written infrastructure (no MCP scaffold tool applies) — this feature is exactly that: a shared lib plus per-page markup/script edits, no `create_page`/`scaffold_list` involved since no new page, model, or handler is created.

## Architecture

**New file:** `src/app/static/js/lib/theme.js` (infrastructure, hand-written)
- `getStoredTheme()` — reads `localStorage.getItem('theme')`, returns `'light' | 'dark' | null`. Wrapped in try/catch; returns `null` on any error (private-mode storage block, etc.).
- `getEffectiveTheme()` — returns stored theme if present, else `matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'`.
- `applyTheme(theme)` — sets `document.documentElement.dataset.theme = theme`; updates `aria-pressed` and icon visibility on any `[data-theme-toggle]` button found in the DOM.
- `initTheme()` — called on each page after DOM ready: applies current effective theme (in case it differs from the head-inline snippet's guess — it won't, but keeps single source of truth), wires click listener on every `[data-theme-toggle]` element to flip theme, persist explicit choice via `localStorage.setItem('theme', ...)`, and re-apply.

**Per-page head snippet** (added to all 7 `.html` files, before the `<link rel="stylesheet">`):
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
Runs synchronously pre-paint so there is no flash of the wrong theme. This is inline but contains no external/user data — reads only `localStorage` and `matchMedia`, consistent with the "no eval on external data" rule.

**Per-page toggle button markup** (added to all 7 `.html` files):
- Desktop header: icon button next to the `#clock` element, `data-theme-toggle`, `aria-label="Toggle dark mode"`, `aria-pressed` reflecting state. Contains two inline `<svg>` icons (sun / moon), toggled via a `hidden` class swap in `theme.js` — no `innerHTML`.
- Mobile drawer: matching button in the drawer footer row next to `#clock-mobile`.

**Per-page script tag:** add `<script type="module" src="/static/js/lib/theme.js"></script>` alongside the existing page-specific module script, calling `initTheme()` at the end of `theme.js` (auto-runs on import, matching the existing page module pattern of calling `init()` at the bottom of the file).

**CSS** (`input.css`): add a dark override block directly after the existing `@theme` block:
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
Because every page already consumes colors exclusively through these custom properties, no other CSS or HTML class changes are needed for the theme to apply everywhere (cards, borders, text, accent links, focus rings, `::selection`, etc.).

## Data flow
1. Page requested → browser parses `<head>` → inline snippet sets `data-theme` attribute on `<html>` before CSS is applied → CSS custom properties resolve to the correct palette on first paint (no flash).
2. `theme.js` loads as a module, runs `initTheme()`: re-applies effective theme (idempotent) and attaches click handlers to toggle button(s) on that page.
3. User clicks toggle → `applyTheme()` flips `data-theme`, `localStorage.setItem('theme', newTheme)`, icon swap.
4. Next page navigation repeats step 1, now reading the persisted explicit choice from `localStorage`.

## Error handling
- All `localStorage` reads/writes wrapped in try/catch. On failure (private browsing, storage disabled), theme falls back to system preference for that load; toggle still works in-memory for the session but won't persist.
- No new failure modes touch the Go backend, DB, or any API — this is a pure static-asset change.

## Testing
No JS test harness exists in this repo. Verification is manual:
1. `docker compose restart app` (recompiles CSS bundle from `input.css`).
2. Load each of the 7 pages, confirm toggle button visible in desktop header and mobile drawer.
3. Click toggle: confirm all surfaces (canvas/surface/raised, borders, text, accent, ok/danger) flip correctly, confirm it persists across a full page reload and across navigating to a different page.
4. Clear `localStorage`, use devtools to emulate `prefers-color-scheme: dark`, reload: confirm page opens in dark mode with no flash of light mode first.
5. Confirm `:focus-visible` outline and `::selection` remain legible in both themes.

## Out of scope
- No per-page or per-section theme overrides — global toggle only.
- No animation/transition polish on the color swap beyond what's already inherited from existing `transition-colors` utility classes on nav links.
- No server-side theme storage or cookie — client-only via `localStorage`.

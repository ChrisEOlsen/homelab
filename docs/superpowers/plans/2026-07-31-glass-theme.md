# Sonoma Glass Theme Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the neon "Uplink" skin with frosted glass panels over a photographic wallpaper, rounded corners, and a single apricot accent — without losing text legibility.

**Architecture:** Almost everything lands in one file, `src/app/static/css/input.css`, via the two levers the last re-skin used: `@theme` token values, and unlayered CSS at the bottom of the file that decorates Tailwind's utility classes (unlayered author CSS outranks `@layer utilities` regardless of specificity). The only markup edits are removing the uplink-rail div and softening label typography.

**Tech Stack:** Tailwind CSS v4 (standalone CLI, no Node), plain ES modules, Go `net/http`. Build is `docker compose restart app`.

## Global Constraints

- **Spec:** `docs/superpowers/specs/2026-07-31-glass-theme-design.md`. Every task's requirements implicitly include it.
- **Branch:** `design/sonoma-glass`. Already created; do not branch again.
- **Never use a git worktree** for this repo (CLAUDE.md) — the MCP container's bind mounts are path-bound.
- **No Node/npm.** Tailwind standalone CLI only.
- **`src/app/static/css/style.css` is a gitignored build artifact.** Only `input.css` is committed. Never `git add` style.css.
- **No JS testing exists in this project** (CLAUDE.md Critical Constraint 4). Verification for every task in this plan is a rebuild plus a browser check — that is the test cycle, and it is not optional.
- **Skin rules stay additive.** Never set `display` or `position` on a shared utility — markup toggles `hidden`/`flex` and relies on positioning ancestors. `background-color` is set on exactly the four chrome selectors named in Task 4, and nowhere else.
- **Ordering matters more than specificity here.** All skin rules are unlayered and mostly specificity 0,2,0, so later rules win. Keep the radius block ordered generic → specific exactly as written in Task 5.
- **Rebuild command:** `docker compose restart app` (recompiles Tailwind *and* the Go binary).
  Faster during iteration: `docker compose exec app /usr/local/bin/tailwindcss -i ./static/css/input.css -o ./static/css/style.css --minify`
- **Serving:** `main.go:45` serves `./static` at `/static/`. The app is at `http://localhost:8080` (or `$APP_PORT`).

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `src/app/static/img/sonoma-evening.jpg` | Wallpaper, ≥900px viewports | 1 |
| `src/app/static/img/sonoma-evening-sm.jpg` | Wallpaper, <900px viewports | 1 |
| `src/app/static/css/input.css` — `@theme` + `:root` blocks | All token values, both themes | 2 |
| `src/app/static/css/input.css` — skin: wallpaper | `body::before` photo + scrim | 3 |
| `src/app/static/css/input.css` — skin: glass | Panel and chrome translucency/blur | 4 |
| `src/app/static/css/input.css` — skin: radius | All corner rounding | 5 |
| `src/app/static/css/input.css` — skin: accents/motion | Neon removal, calmed LED | 6 |
| `src/app/static/pages/*.html` (7 files) | Rail removal, preload links, typography | 6, 7 |
| `src/app/static/js/{home,todos,reminders}.js` | Label/button class strings | 7 |

---

### Task 1: Background assets

**Files:**
- Create: `src/app/static/img/sonoma-evening.jpg`
- Create: `src/app/static/img/sonoma-evening-sm.jpg`

**Interfaces:**
- Consumes: `Sonoma Evening 1.png` at the repo root (3840×2160, 16-bit, 34 MB, untracked — it stays untracked).
- Produces: two URLs used by Task 3 — `/static/img/sonoma-evening.jpg` and `/static/img/sonoma-evening-sm.jpg`.

- [ ] **Step 1: Confirm the app container is running**

```bash
docker compose ps
```
Expected: an `app` service listed as running. If not: `docker compose up -d`.

- [ ] **Step 2: Generate both JPEGs**

`sips` is the macOS built-in; no dependency to install.

```bash
mkdir -p src/app/static/img
sips -Z 2560 "Sonoma Evening 1.png" --setProperty format jpeg --setProperty formatOptions 72 \
  --out src/app/static/img/sonoma-evening.jpg
sips -Z 1600 "Sonoma Evening 1.png" --setProperty format jpeg --setProperty formatOptions 70 \
  --out src/app/static/img/sonoma-evening-sm.jpg
```

- [ ] **Step 3: Verify size and dimensions**

```bash
ls -lh src/app/static/img/
sips -g pixelWidth -g pixelHeight src/app/static/img/sonoma-evening.jpg
```
Expected: `sonoma-evening.jpg` ≈ 650–700 KB at 2560×1440; `sonoma-evening-sm.jpg` ≈ 260–280 KB at 1600×900. If either is more than ~1.5× those sizes, drop the quality by 5 and regenerate — the page must not ship a multi-megabyte wallpaper.

- [ ] **Step 4: Verify the app serves them**

```bash
curl -s -o /dev/null -w '%{http_code} %{size_download}\n' http://localhost:8080/static/img/sonoma-evening.jpg
```
Expected: `200` and a byte count matching the file. A `404` means the bind mount didn't pick up the new directory — `docker compose restart app` and retry.

- [ ] **Step 5: Commit**

```bash
git add src/app/static/img/
git commit -m "feat(theme): add Sonoma Evening wallpaper assets"
```

---

### Task 2: Token layer

Replace the `@theme` block, the `:root` block and the `:root[data-theme="dark"]` block (currently `input.css:44-128`) wholesale. Leave everything from `@layer base` down untouched in this task.

**Files:**
- Modify: `src/app/static/css/input.css:5-128` (the header comment and all three token blocks)

**Interfaces:**
- Produces, for Tasks 3–6: `--color-backdrop`, `--color-surface-chrome`, `--scrim`, `--panel-blur`, `--chrome-blur`, `--panel-sheen`, `--panel-shadow`, `--row-shadow`, `--r-panel`, `--r-row`, `--r-well`, `--r-dialog`, `--r-chip`, `--halo-accent`.
- Note: `--glow-accent`, `--glow-soft`, `--text-glow`, `--grad-violet`, `--grad-azure`, `--grad-cyan`, `--title-start`, `--ambient-1..3` are all **deleted**. Task 6 removes their last uses. Between Task 2 and Task 6 the stylesheet references undefined vars — that is expected and self-corrects; do not "fix" it by reintroducing them.

- [ ] **Step 1: Replace the header comment**

Replace lines 5–43 (the `/* Homelab design tokens — "Uplink" ... */` block) with:

```css
/*
 * Homelab design tokens — "Sonoma"
 * --------------------------------
 * Direction: frosted glass over a photograph. Panels are translucent
 * enough that the wallpaper reads through them as color and shape, but
 * never as detail — legibility outranks the effect everywhere it is a
 * choice between the two. Depth is still three layers: canvas < surface
 * < surface-raised.
 *
 * The one accent is sunset apricot, sampled from the horizon in the
 * wallpaper. It is flat: no gradients, no glow, no emitted light.
 *
 * Reuse these tokens on every page for a consistent system:
 *   bg-canvas / bg-surface / bg-surface-raised
 *   text-ink / text-ink-dim
 *   border-hairline
 *   text-accent / bg-accent / border-accent / ring-accent
 *   text-ok / bg-ok / border-ok
 *   text-danger / bg-danger / border-danger
 *
 * Note: the page-background color is named "canvas", not "base" —
 * "base" would collide with Tailwind's built-in text-base (font-size)
 * utility and produce a confusing/ambiguous class name.
 *
 * Note: the hairline/border token is named "--color-hairline" (kept
 * from earlier themes) rather than "--color-border" — only the value
 * has ever changed, so pages already referencing border-hairline /
 * divide-hairline keep working untouched.
 *
 * Note: `canvas` is OPAQUE and must stay that way. It is not only the
 * page fill — it is the background of every input well and code block
 * (42 uses), and `text-canvas` is the LABEL COLOR on accent buttons.
 * Give it alpha and every button label goes see-through. The page
 * background is `--color-backdrop`, a separate non-utility var.
 *
 * Note: surface / surface-raised carry alpha. Tailwind's slash-opacity
 * modifier (bg-surface/50) would compound with that alpha, so it is not
 * used anywhere in this app — use the token as-is.
 */
```

- [ ] **Step 2: Replace the `@theme` block (light values)**

```css
@theme {
  /* Light — the same glass with the sun on it. Panels go white-frosted
     and the accent darkens well past the sky's literal color, because
     apricot-on-white has to clear contrast as text and as a border. */
  --color-canvas: #ffffff;
  --color-surface: rgb(255 255 255 / 0.72);
  --color-surface-raised: rgb(255 255 255 / 0.52);
  --color-hairline: rgb(28 46 38 / 0.16);
  --color-ink: #16211c;
  --color-ink-dim: #55635c;
  --color-accent: #b85c18;
  --color-accent-dim: #d98b4a;
  --color-ok: #16714a;
  --color-danger: #b3243f;
}
```

- [ ] **Step 3: Replace the `:root` block (light non-utility vars)**

```css
/*
 * Non-utility design variables
 * ----------------------------
 * These do not generate Tailwind classes — they feed the skin rules at
 * the bottom of this file (wallpaper, scrim, glass depth, radius).
 * Split light/dark the same way the colors are.
 */
:root {
  /* Painted on <html>, behind the wallpaper. Its only job is to be the
     first paint before the JPEG decodes, so it must not be white-hot
     or the load reads as a flash. */
  --color-backdrop: #e9e4dc;

  /* Chrome — header, drawer, dialogs — is glass, but the least
     transparent glass on the page. It floats over content AND over the
     wallpaper's bright sky band, so it is where text sits closest to
     noise. */
  --color-surface-chrome: rgb(255 255 255 / 0.86);
  --color-surface-solid: #ffffff;

  /* The scrim sits between the photo and the content. Top-weighted,
     because the photo's apricot sky band is its brightest region and
     the sticky header lands right on it. */
  --scrim:
    linear-gradient(
      180deg,
      rgb(255 255 255 / 0.66) 0%,
      rgb(255 251 246 / 0.48) 30%,
      rgb(255 255 255 / 0.38) 100%
    ),
    radial-gradient(
      130% 95% at 50% 38%,
      transparent 42%,
      rgb(255 255 255 / 0.34) 100%
    );

  --panel-blur: blur(28px) saturate(180%);
  --chrome-blur: blur(34px) saturate(190%);

  --panel-sheen: rgb(255 255 255 / 0.55);
  --panel-shadow:
    inset 0 1px 0 rgb(255 255 255 / 0.9),
    0 18px 44px -30px rgb(20 40 30 / 0.38);
  --row-shadow: inset 0 1px 0 rgb(255 255 255 / 0.6);

  --halo-accent: rgb(184 92 24 / 0.30);

  /* Corner radius. One scale, referenced by the radius block below. */
  --r-panel: 18px;
  --r-row: 12px;
  --r-well: 10px;
  --r-dialog: 20px;
  --r-chip: 4px;
}
```

- [ ] **Step 4: Replace the `:root[data-theme="dark"]` block**

```css
:root[data-theme="dark"] {
  --color-canvas: #0b1210;
  --color-surface: rgb(20 28 26 / 0.72);
  --color-surface-raised: rgb(122 140 132 / 0.16);
  --color-hairline: rgb(255 255 255 / 0.14);
  --color-ink: #f1f5f2;
  --color-ink-dim: #a6b3ac;
  --color-accent: #ffb067;
  --color-accent-dim: #c9762f;
  --color-ok: #6fdba0;
  --color-danger: #ff8080;

  --color-backdrop: #0a100e;

  --color-surface-chrome: rgb(16 23 21 / 0.85);
  --color-surface-solid: #131a18;

  --scrim:
    linear-gradient(
      180deg,
      rgb(6 11 9 / 0.74) 0%,
      rgb(6 11 9 / 0.52) 30%,
      rgb(6 11 9 / 0.44) 100%
    ),
    radial-gradient(
      130% 95% at 50% 38%,
      transparent 42%,
      rgb(4 8 7 / 0.48) 100%
    );

  --panel-blur: blur(28px) saturate(180%);
  --chrome-blur: blur(34px) saturate(190%);

  --panel-sheen: rgb(255 255 255 / 0.07);
  --panel-shadow:
    inset 0 1px 0 rgb(255 255 255 / 0.16),
    0 22px 50px -30px rgb(0 0 0 / 0.9);
  --row-shadow: inset 0 1px 0 rgb(255 255 255 / 0.08);

  --halo-accent: rgb(255 176 103 / 0.34);

  --r-panel: 18px;
  --r-row: 12px;
  --r-well: 10px;
  --r-dialog: 20px;
  --r-chip: 4px;
}
```

- [ ] **Step 5: Rebuild and confirm the CSS compiles**

```bash
docker compose exec app /usr/local/bin/tailwindcss -i ./static/css/input.css -o ./static/css/style.css --minify
```
Expected: no errors. The page will look broken at this stage (neon rules still reference deleted vars) — that is expected and Task 6 resolves it. Do not chase it here.

- [ ] **Step 6: Commit**

```bash
git add src/app/static/css/input.css
git commit -m "feat(theme): retune tokens to the Sonoma glass palette"
```

---

### Task 3: Wallpaper and scrim

**Files:**
- Modify: `src/app/static/css/input.css` — the `html { background-color }` rule (~line 173) and the `body::before` rule (~line 203)

**Interfaces:**
- Consumes: `--color-backdrop`, `--scrim` (Task 2); the two image URLs (Task 1).
- Produces: nothing later tasks depend on.

- [ ] **Step 1: Repoint the `<html>` fill**

Replace:
```css
html {
  background-color: var(--color-canvas);
}
```
with:
```css
html {
  background-color: var(--color-backdrop);
}
```

Leave the second `html { scrollbar-width: none; }` rule and the `::-webkit-scrollbar` rules exactly as they are — hiding the page scrollbar is unrelated to this change and still wanted.

Leave `body.bg-canvas { background-color: transparent; }` alone. It is what stops the now-opaque canvas token from painting over the wallpaper.

- [ ] **Step 2: Replace the ambient-field pseudo-element with the wallpaper**

Replace the whole `body::before` rule (and update the comment above it) with:

```css
/*
 * Wallpaper.
 * A fixed pseudo-element rather than `background-attachment: fixed`,
 * which iOS Safari ignores and desktop repaints expensively on scroll.
 * body is `flex flex-col`, but a fixed-position pseudo-element is out
 * of flow and never becomes a flex item.
 *
 * The scrim is an earlier background layer in the same element, so it
 * paints ON TOP of the photo — background layers stack first-on-top.
 * One element, two layers, no extra DOM.
 */
body::before {
  content: "";
  position: fixed;
  inset: 0;
  z-index: -1;
  pointer-events: none;
  background-image: var(--scrim), url("/static/img/sonoma-evening.jpg");
  background-size: auto, cover;
  background-position: center, center;
  background-repeat: no-repeat, no-repeat;
}

/*
 * Phones get the 1600px file — a quarter the bytes. This is a media
 * query, not image-set(): image-set() selects on pixel density, which
 * cannot express "smaller viewport".
 */
@media (max-width: 899px) {
  body::before {
    background-image: var(--scrim), url("/static/img/sonoma-evening-sm.jpg");
  }
}
```

- [ ] **Step 3: Rebuild and look at it**

```bash
docker compose exec app /usr/local/bin/tailwindcss -i ./static/css/input.css -o ./static/css/style.css --minify
```
Open `http://localhost:8080/static/pages/home.html`.
Expected: the photo is visible behind the page, fixed while content scrolls. Panels are still neon-shaped — fine. Confirm specifically: scrolling does not drag the photo, and there is no white band at the bottom on a short page.

- [ ] **Step 4: Check the mobile asset switches**

Narrow the browser below 900px (or DevTools device mode). In DevTools → Network, reload and confirm `sonoma-evening-sm.jpg` is requested and `sonoma-evening.jpg` is not.

- [ ] **Step 5: Commit**

```bash
git add src/app/static/css/input.css
git commit -m "feat(theme): render the wallpaper and scrim behind the page"
```

---

### Task 4: Glass

**Files:**
- Modify: `src/app/static/css/input.css` — the `.bg-surface` blur rule and the chrome-opacity rules (~lines 216–277)

**Interfaces:**
- Consumes: `--panel-blur`, `--chrome-blur`, `--color-surface-chrome`, `--color-surface-solid`, `--panel-shadow`, `--row-shadow`, `--panel-sheen` (Task 2).

- [ ] **Step 1: Replace the blur and chrome block**

Replace everything from the `/* Backlit glass. ... */` comment through the `header nav.bg-surface { ... }` rule with:

```css
/* Frosted glass. Only `.bg-surface` blurs — `.bg-surface-raised` rows
   already sit on top of a blurred panel, so blurring them again costs
   compositing for no visible gain. With 124 raised elements in the app,
   that matters on iOS. */
.bg-surface {
  backdrop-filter: var(--panel-blur);
  -webkit-backdrop-filter: var(--panel-blur);
}

/* Chrome floats over content and over the wallpaper's bright sky band,
   so it is glass at a higher alpha and a stronger blur than the panels
   in the page. This is the one place a skin rule overrides a utility's
   background-color, and it is deliberately scoped. */
header.bg-surface,
aside.bg-surface,
[role="dialog"].bg-surface {
  background-color: var(--color-surface-chrome);
  backdrop-filter: var(--chrome-blur);
  -webkit-backdrop-filter: var(--chrome-blur);
}

/* The kebab dropdown stays effectively opaque: it is small, it pops
   over dense list rows, and translucency buys it nothing. Specificity
   0,2,0 beats the 0,1,1 chrome selectors above regardless of order. */
.absolute.bg-surface {
  background-color: var(--color-surface-solid);
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

/* The segmented nav rides on already-blurred chrome, so it has nothing
   left to blur — but it keeps its translucency, which reads as a raised
   control against the chrome behind it. */
header nav.bg-surface {
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}
```

- [ ] **Step 2: Keep the panel-depth rules, retuned**

The `.border.bg-surface`, `.border.bg-surface-raised`, `header.bg-surface`, `aside.bg-surface` and `[role="dialog"].bg-surface` box-shadow rules that follow stay as they are — they already read from `--panel-shadow` / `--row-shadow` / `--panel-sheen`, which Task 2 retuned. The one edit: the sheen gradient's stop is tuned for a square panel; soften it so it does not band against a rounded corner.

Replace:
```css
  background-image: linear-gradient(180deg, var(--panel-sheen), transparent 96px);
```
with:
```css
  background-image: linear-gradient(180deg, var(--panel-sheen), transparent 140px);
```

- [ ] **Step 3: Rebuild and check contrast**

```bash
docker compose exec app /usr/local/bin/tailwindcss -i ./static/css/input.css -o ./static/css/style.css --minify
```
Open `http://localhost:8080/static/pages/home.html`.
Expected: panels are frosted; the photo reads through as color, not detail. Check the header specifically — it sits on the bright sky band, and its text must be crisply readable. If it is not, raise `--color-surface-chrome` alpha (both themes) to 0.90 before continuing.

- [ ] **Step 4: Commit**

```bash
git add src/app/static/css/input.css
git commit -m "feat(theme): frost the panels and glass the chrome"
```

---

### Task 5: Corners

**Files:**
- Modify: `src/app/static/css/input.css` — insert a new radius block after the glass block from Task 4

**Interfaces:**
- Consumes: `--r-panel`, `--r-row`, `--r-well`, `--r-dialog`, `--r-chip` (Task 2).

- [ ] **Step 1: Insert the radius block**

Nothing in the markup uses `rounded-*` except `rounded-full`, so every corner in the app comes from here. Insert verbatim — **the order is load-bearing.** These rules are all unlayered and mostly specificity 0,2,0, so later rules win ties.

```css
/* ------------------------------------------------------------------
 * Corners
 * ------------------------------------------------------------------
 * Ordered generic → specific on purpose. Every rule below is unlayered
 * and most are specificity 0,2,0, so ties are broken by source order,
 * not by weight. Moving a rule up or down changes the result.
 * ------------------------------------------------------------------ */

/* Base: anything with a full border is a box — buttons, inputs, code
   blocks, tables, chips. */
.border {
  border-radius: var(--r-well);
}

/* A table with border-collapse cannot round its own corners; the cells
   square them off again and you get a visible mismatch. */
table.border {
  border-radius: 0;
}

.border.bg-surface-raised {
  border-radius: var(--r-row);
}

.border.bg-surface {
  border-radius: var(--r-panel);
}

/* Inline code chips are ~16px tall; the well radius swallows them. */
.bg-canvas.px-1 {
  border-radius: var(--r-chip);
}

[role="dialog"].bg-surface {
  border-radius: var(--r-dialog);
}

/* The drawer slides in from the right, so only its left corners are
   ever visible. */
aside.bg-surface {
  border-radius: var(--r-dialog) 0 0 var(--r-dialog);
}

/* The segmented nav becomes a pill. `overflow: hidden` is required, not
   decorative: the segments' hover fills (hover:bg-surface-raised) paint
   past a rounded parent's corners without it. */
header nav.bg-surface {
  border-radius: 999px;
  overflow: hidden;
}

/* Same reason — the dropdown's hovered rows would square its corners. */
.absolute.bg-surface {
  border-radius: var(--r-row);
  overflow: hidden;
}

/* Filled accent buttons that carry no `.border`. */
.bg-accent:not(.rounded-full) {
  border-radius: var(--r-well);
}

/* Re-assert pills LAST. `rounded-full` is a Tailwind utility, so it
   lives in @layer utilities — and every unlayered rule above outranks
   it regardless of specificity. Without this line the 22 status dots
   and pills in the app quietly become squares. */
.rounded-full {
  border-radius: 9999px;
}
```

- [ ] **Step 2: Rebuild**

```bash
docker compose exec app /usr/local/bin/tailwindcss -i ./static/css/input.css -o ./static/css/style.css --minify
```

- [ ] **Step 3: Verify the pills survived**

Open `http://localhost:8080/static/pages/home.html`. Check, in this order:
1. The LED dot next to the wordmark is a **circle**, not a rounded square. (This is the `rounded-full` regression the last rule guards.)
2. The desktop nav is a pill and its hover highlight is clipped inside the pill's ends.
3. Panels are 18px, rows inside them 12px, inputs 10px.
4. Open the Add Todo dialog on `/static/pages/todos.html` — 20px corners, and its hover states stay inside.

- [ ] **Step 4: Commit**

```bash
git add src/app/static/css/input.css
git commit -m "feat(theme): round every corner in the system"
```

---

### Task 6: Strip the neon

This is the task that makes the page stop looking like the old theme. It removes the last references to the vars Task 2 deleted, so the stylesheet is internally consistent again at the end of it.

**Files:**
- Modify: `src/app/static/css/input.css` — the `.uplink-rail` rules, `.bg-accent` / `.text-accent` / `.border-accent` / hover rules, `.page-title`, `.led-pulse`, the `prefers-reduced-motion` block
- Modify: `src/app/static/pages/*.html` (all 7) — remove the rail div

**Interfaces:**
- Consumes: `--halo-accent` (Task 2).
- Produces: `.page-title` survives as a class name in the markup but now means "solid heading with a legibility shadow" rather than "gradient fill". Do not remove the class from the HTML.

- [ ] **Step 1: Delete the uplink rail from the stylesheet**

Delete the entire block from the `/* Signature: the uplink rail. ... */` comment through the closing brace of `@keyframes uplink-sweep` (~lines 279–318). Nothing replaces it.

- [ ] **Step 2: Delete the rail div from all 7 pages**

It is line 62 of every page, identical in each:

```bash
cd src/app/static/pages
sed -i '' '/<div class="uplink-rail" aria-hidden="true"><\/div>/d' *.html
grep -rn "uplink" . ; echo "exit=$?"
```
Expected: the grep prints nothing and `exit=1` (no matches). Anything else means a page was missed.

- [ ] **Step 3: Flatten the accent rules**

Replace the whole run from the `/* Accent surfaces emit. ... */` comment through the `.hover\:border-accent:hover, .border.bg-surface-raised:hover { ... }` rule with:

```css
/* The accent is flat. No gradient fill, no emitted glow — apricot is
   the light in the photograph, and the UI does not need to also glow.
   `.bg-accent` deliberately sets no background-image, so the utility
   keeps sole ownership of the fill. */
.hover\:bg-accent:hover {
  color: var(--color-canvas);
}

/* Rows lift on hover instead of igniting. */
.hover\:border-accent:hover,
.border.bg-surface-raised:hover {
  box-shadow:
    var(--row-shadow),
    0 6px 18px -12px rgb(0 0 0 / 0.4);
}
```

- [ ] **Step 4: Replace the gradient page title**

Replace the whole `@supports (background-clip: text) ... { .page-title { ... } }` block with:

```css
/*
 * Page titles are the one piece of text with no panel behind it — the
 * <h1> sits directly on the wallpaper. Solid ink plus a soft shadow to
 * hold it against whatever the photo is doing underneath.
 */
.page-title {
  text-shadow: 0 1px 12px rgb(0 0 0 / 0.28);
}

:root:not([data-theme="dark"]) .page-title {
  text-shadow: 0 1px 10px rgb(255 255 255 / 0.65);
}
```

- [ ] **Step 5: Calm the LED**

Replace the `@keyframes led-pulse { ... }` block and the `.led-pulse { ... }` rule with:

```css
/* The header beacon. Steady, not throbbing — a lit indicator rather
   than a blinking link-activity LED. */
.led-pulse {
  box-shadow:
    0 0 0 3px var(--halo-accent),
    0 0 12px 0 var(--halo-accent);
}
```

- [ ] **Step 6: Update the reduced-motion block**

Both animations it names are now gone. Replace:

```css
@media (prefers-reduced-motion: reduce) {
  .led-pulse,
  .uplink-rail::after {
    animation: none;
  }
  * {
```
with:

```css
@media (prefers-reduced-motion: reduce) {
  * {
```

Leave the rest of that block — the `transition-duration` / `animation-duration` / `animation-iteration-count` overrides — exactly as it is. The `animation-iteration-count: 1 !important` line in particular is load-bearing and its comment explains why.

- [ ] **Step 7: Confirm no dead variable references remain**

```bash
cd src/app/static/css
grep -n "glow-accent\|glow-soft\|text-glow\|grad-violet\|grad-azure\|grad-cyan\|title-start\|ambient-" input.css ; echo "exit=$?"
```
Expected: nothing printed, `exit=1`. Any hit is a rule still reading a variable Task 2 deleted — it will render as `unset` and silently look wrong. Fix it before committing.

- [ ] **Step 8: Rebuild and verify**

```bash
docker compose exec app /usr/local/bin/tailwindcss -i ./static/css/input.css -o ./static/css/style.css --minify
```
Open home. Expected: no sweeping light under the header; the page title is solid, not gradient-filled; accent buttons are flat apricot with no halo; the LED is a steady dot. The page should now read as glass rather than neon.

- [ ] **Step 9: Commit**

```bash
git add src/app/static/css/input.css src/app/static/pages/
git commit -m "feat(theme): strip the neon signatures"
```

---

### Task 7: Typography and preload

**Files:**
- Modify: `src/app/static/pages/*.html` (all 7)
- Modify: `src/app/static/js/home.js`, `src/app/static/js/todos.js`, `src/app/static/js/reminders.js`

**Interfaces:**
- Consumes: nothing. This task is independent of Tasks 2–6 and could run first; it is last because it is the only one touching JS.

The header and drawer markup is byte-identical across all 7 pages (same content on the same line numbers), so scripted edits are safe and uniform. Verify with the greps in Step 5 rather than trusting that.

- [ ] **Step 1: Sentence-case the nav and chrome labels**

```bash
cd src/app/static/pages
sed -i '' \
  -e 's|>DASHBOARD<|>Dashboard<|g' \
  -e 's|>REMINDERS<|>Reminders<|g' \
  -e 's|>TASKS<|>Tasks<|g' \
  -e 's|>JOURNAL<|>Journal<|g' \
  -e 's|>CODEX<|>Codex<|g' \
  -e 's|>BOOKMARKS<|>Bookmarks<|g' \
  -e 's|>LOGGER<|>Logger<|g' \
  -e 's|>MENU<|>Menu<|g' \
  -e 's|^\( *\)HOMELAB$|\1Homelab|' \
  *.html
```

- [ ] **Step 2: Drop the LOCAL label and the wide tracking**

`LOCAL` is a redundant label on the clock — the time stands alone. It appears twice per page (desktop header, mobile drawer) as its own line:

```bash
cd src/app/static/pages
sed -i '' '/^ *<span>LOCAL<\/span>$/d' *.html
sed -i '' \
  -e 's|text-xs tracking-widest text-ink-dim|text-xs text-ink-dim|g' \
  -e 's|text-sm tracking-wide shrink-0|text-sm font-medium shrink-0|g' \
  -e 's|text-xs tracking-wide overflow-x-auto|text-xs overflow-x-auto|g' \
  -e 's|text-sm tracking-wide overflow-y-auto|text-sm overflow-y-auto|g' \
  *.html
```

- [ ] **Step 3: De-shout the one section eyebrow**

`home.html:125` is the only `uppercase` in any page.

```bash
cd src/app/static/pages
sed -i '' 's|class="text-xs text-ink-dim uppercase"|class="text-xs font-medium text-ink-dim"|' home.html
```

Note this depends on Step 2 having already rewritten `text-xs tracking-widest text-ink-dim` → `text-xs text-ink-dim` in that same class attribute. If Step 2 was skipped, this sed silently matches nothing — check the grep in Step 5.

- [ ] **Step 4: Soften the JS-built form labels and buttons**

16 occurrences across three files, all the same fragment:

```bash
cd src/app/static/js
sed -i '' 's|text-xs uppercase tracking-wide|text-xs font-medium|g' home.js todos.js reminders.js
```

- [ ] **Step 5: Verify nothing shouty survives**

```bash
cd src/app/static
grep -rn "uppercase\|tracking-wide\|tracking-widest" pages js ; echo "exit=$?"
grep -rn "HOMELAB\|>MENU<\|LOCAL" pages ; echo "exit=$?"
```
Expected: both print nothing, `exit=1` each. `tracking-tight` on the `<h1>` is a different class and correctly does not match `tracking-wide`.

- [ ] **Step 6: Add the wallpaper preload to all 7 pages**

Each page's `<head>` ends with the stylesheet link. Insert two preloads after it — one per breakpoint, so a phone never fetches the 672 KB file:

```bash
cd src/app/static/pages
sed -i '' 's|^\( *\)<link rel="stylesheet" href="/static/css/style.css">$|\1<link rel="preload" as="image" href="/static/img/sonoma-evening.jpg" media="(min-width: 900px)">\
\1<link rel="preload" as="image" href="/static/img/sonoma-evening-sm.jpg" media="(max-width: 899px)">\
\1<link rel="stylesheet" href="/static/css/style.css">|' *.html
grep -c "rel=\"preload\"" *.html
```
Expected: every file reports `2`.

- [ ] **Step 7: Rebuild and check every page**

```bash
docker compose restart app
```
(Full restart, not just the Tailwind CLI — Tailwind scans the HTML for class names, and Steps 1–3 changed them.)

Open all seven and confirm the nav reads `Dashboard / Reminders / Tasks / Journal / Codex / Bookmarks / Logger` in sentence case with normal tracking, and that the clock still shows.

- [ ] **Step 8: Commit**

```bash
git add src/app/static/pages/ src/app/static/js/
git commit -m "feat(theme): soften the type to sentence case, preload the wallpaper"
```

---

### Task 8: Full verification pass

**Files:** none modified unless a defect is found.

- [ ] **Step 1: Full rebuild from clean**

```bash
docker compose restart app
docker compose logs --tail=30 app
```
Expected: Tailwind recompiles, the Go binary builds, the server listens. No errors.

- [ ] **Step 2: Go tests still pass**

Nothing in this change touches Go, but CLAUDE.md requires the check alongside the logs.

```bash
docker compose exec app go test ./...
```
Expected: all packages `ok` or `no test files`.

- [ ] **Step 3: Walk all 7 pages in both themes**

Pages: `home`, `reminders`, `todos`, `journal`, `codex`, `bookmarks`, `logger` at `http://localhost:8080/static/pages/<name>.html`. Toggle with the sun/moon button in the header. For each, check in this order — these are where glass legibility fails first:

1. Small `text-ink-dim` text — timestamps, hints, metadata — on a panel over the photo's bright sky band.
2. Accent-colored text and borders **in light mode** (apricot on white frosted glass is the tightest contrast in the system).
3. Accent-button labels: `text-canvas` on `bg-accent`, both themes. They must be fully opaque — a see-through label means `--color-canvas` picked up alpha somewhere.
4. The header and desktop nav where they overlap the sky band.

- [ ] **Step 4: Check the interactive surfaces**

- Open the Add/Edit Todo dialog (todos) — rounded, glass, readable over the scrim.
- Open a kebab dropdown (todos or bookmarks) — opaque, rounded, hover rows clipped.
- Open the mobile drawer below 768px — rounded left corners, readable.
- Expand/collapse subtasks on todos — the height transition still animates.
- Complete a task — the clear-out collapse animation still plays.

- [ ] **Step 5: Confirm all 22 pills are still circles**

Spot-check the header LED, the nav's current-page dot, and any status dots in lists. Any rounded-square is a `rounded-full` regression from Task 5.

- [ ] **Step 6: Mobile pass**

At <900px: the small JPEG loads, the layout stays legible at `--spacing: 0.2rem`, and the rounded corners do not collide with the tightened padding.

- [ ] **Step 7: Confirm the build artifact was never committed**

```bash
git status --porcelain
git log --stat --oneline design/sonoma-glass ^main | grep "style.css" ; echo "exit=$?"
```
Expected: a clean tree, and the grep prints nothing (`exit=1`). `style.css` is gitignored and must stay out of every commit.

---

## Self-Review

**Spec coverage:**

| Spec section | Task |
|---|---|
| 1. Background asset | 1 |
| 2. Wallpaper rendering (photo, scrim, backdrop, preload) | 3, and preload in 7 |
| 3. Tokens (surface, raised, canvas-opaque, accent, hairline) | 2 |
| 4. Corners (incl. both traps) | 5 |
| 5. Chrome | 4 |
| 6. Typography | 7 |
| 7. Motion (rail, page-title, LED, boot-rise kept) | 6 |
| 8. Text not on a panel (h1, footer, empty states) | 6 (h1); **see note** (footer) |
| 9. Non-goals | respected throughout |
| Verification | 8 |

**Gap found and closed:** spec §8 calls for the footer to get glass so its `text-ink-dim` label stays readable over the photo. The footer markup is `border-t border-hairline px-4 sm:px-6 py-4 text-xs text-ink-dim` — it has **no `bg-surface`**, so no task above touched it. Task 6 gains a step for it, below.

**Placeholder scan:** no TBDs, no "add error handling", no "similar to Task N". Every code step carries the literal content.

**Type consistency:** variable names cross-checked between Task 2's definitions and their uses in Tasks 3–6 — `--color-backdrop`, `--scrim`, `--color-surface-chrome`, `--panel-blur`, `--chrome-blur`, `--halo-accent`, `--r-*` all defined before use, in both themes. Task 6 Step 7's grep is the mechanical check that nothing reads a deleted var.

### Task 6, Step 5b: Glass the footer

Insert after the `.led-pulse` rule:

```css
/* The footer is the only chrome with no fill of its own — it would
   otherwise put text-ink-dim directly on the photograph. */
footer {
  background-color: var(--color-surface-chrome);
  backdrop-filter: var(--chrome-blur);
  -webkit-backdrop-filter: var(--chrome-blur);
}
```

This is a bare element selector, so it cannot collide with a utility class, and it sets `background-color` on an element that had none rather than overriding one.

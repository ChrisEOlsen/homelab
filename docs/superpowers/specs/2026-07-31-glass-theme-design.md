# Design: "Sonoma" glass theme

**Date:** 2026-07-31
**Replaces:** the "Uplink" neon-blue theme (2026-07-28)

## Goal

Re-skin the homelab in the glass idiom of macOS Tahoe / iOS: frosted translucent
panels over a photographic wallpaper, generous rounded corners, and a single warm
accent. Legibility is the governing constraint — the photo is decoration, and no
amount of glass may cost text contrast.

## The one lever

`src/app/static/css/input.css` drives the whole look through two mechanisms, and
this change uses the same two. Neither requires touching the ~5,900 lines of
JS-built DOM:

1. **`@theme` token values.** Every page and every JS-generated element already
   uses only `bg-canvas / bg-surface / bg-surface-raised / border-hairline /
   text-ink / text-ink-dim / text-accent / bg-accent / border-accent / ok /
   danger`.
2. **Unlayered CSS at the bottom of the file.** Tailwind ships utilities inside
   `@layer utilities`; unlayered author CSS outranks layered author CSS
   regardless of specificity, so a plain `.bg-surface { … }` rule decorates the
   utility with no `!important` and no specificity escalation.

The typography change (section 6) is the only part that edits HTML and JS.

---

## 1. Background asset

The source `Sonoma Evening 1.png` (3840×2160, 16-bit, 34 MB) is a master, not a
web asset. It stays at the repo root, **untracked**. Two derived JPEGs are
committed:

| File | Dimensions | Quality | Size | Served to |
|---|---|---|---|---|
| `src/app/static/img/sonoma-evening.jpg` | 2560×1440 | 72 | ~672 KB | viewports ≥ 900px |
| `src/app/static/img/sonoma-evening-sm.jpg` | 1600×900 | 70 | ~265 KB | viewports < 900px |

Generated with macOS `sips`. Selection is by `@media (max-width: 899px)`
overriding the `background-image` — not `image-set()`, whose resolution-based
selection does not express "smaller viewport".

Static files are served by `main.go:45` from `./static`, so the URL is
`/static/img/sonoma-evening.jpg`.

## 2. Wallpaper rendering

The photo renders in the existing `body::before` fixed pseudo-element that
currently holds the neon ambient gradients. Keeping that element rather than
adding one preserves the properties it was chosen for: it is viewport-fixed
without `background-attachment: fixed` (which iOS Safari ignores and desktop
repaints expensively on scroll), it is out of flow so it never becomes a flex
item of the `flex flex-col` body, and it sits at `z-index: -1`.

The scrim rides in the same element as an earlier background layer, above the
photo:

```
background-image:
  <scrim gradients>,          /* painted on top */
  url(/static/img/sonoma-evening.jpg);   /* painted underneath */
background-size: auto, cover;
background-position: center, center;
```

Scrim composition:

- A **top-weighted darkening gradient** over roughly the first 40% of the
  viewport height. The photo's apricot sky band occupies the top ~15% of the
  frame and is its brightest region; the sticky header sits directly on it.
- A **soft overall vignette** to settle the mid-field.

In light mode the scrim brightens instead of darkens, using the same geometry.

`<html>` keeps an opaque fill (a deep near-neutral in dark, near-white in light)
so first paint before the JPEG decodes is not a white flash. Each of the 7 page
heads gets `<link rel="preload" as="image" href="/static/img/sonoma-evening.jpg">`.

## 3. Tokens

### `surface` — frosted panel glass

Alpha ≈ 0.72. Blur increases from 14px to ~28px and saturation from 130–150% to
~180%. The saturation boost is what distinguishes frosted glass from fog: it
pulls the field's green and the sky's apricot through as color rather than grey.

### `surface-raised` — the tint on top

A lighter tint at similar alpha. **Never blurred.** It always sits on an
already-blurred `.bg-surface` panel, so a second `backdrop-filter` costs
compositing for no visible gain — this rule exists today and is retained. With
124 `bg-surface-raised` elements in the app, it matters on iOS.

### `canvas` — stays fully opaque

Non-obvious and load-bearing. `bg-canvas` is not only the page background:

- it fills **input wells and code blocks** (42 uses, e.g.
  `mt-1 block w-full bg-canvas border border-hairline px-3 py-2 …`), and
- `text-canvas` is the **label color on accent buttons**
  (`bg-accent text-canvas`).

Making canvas translucent would make every accent-button label see-through.
Keeping it opaque also gives text fields a solid well inside the glass panel,
which is what iOS does. The existing `body.bg-canvas { background-color:
transparent }` skin rule already stops this from filling the page, and stays.

### `accent` — sunset apricot

Replaces electric azure. One primary; no gradient endpoints.

| | dark | light |
|---|---|---|
| `--color-accent` | `#FFB067` | `#B85C18` |

The light value is pushed well past the sky's literal color because it must clear
contrast as text and as a border on white frosted glass.

Deleted with it: the `azure → cyan` `background-image` on `.bg-accent`, the
`hover:bg-accent` gradient sweep, every `box-shadow` glow
(`--glow-accent`, `--glow-soft`), and `--text-glow` on `.text-accent`.

### `hairline` — the glass edge

The single detail that most reads as glass: a bright 1px inner highlight along
the top edge of each panel, dimming toward the sides. Implemented as the existing
`--panel-sheen` / `--panel-shadow` `inset 0 1px 0` pair, retuned — the sheen is a
`background-image` gradient so the `bg-surface` utility keeps ownership of
`background-color` underneath it.

## 4. Corners

Nothing in the markup uses `rounded-*` today except `rounded-full` (22 status
dots and pills), so radius comes entirely from skin rules on the token classes:

| Target | Selector | Radius |
|---|---|---|
| Panels | `.border.bg-surface` | 18px |
| Rows / cards | `.border.bg-surface-raised` | 12px |
| Wells (inputs, code) | `.border.bg-canvas` | 10px |
| Inline code chips | `.px-1.bg-canvas` | 4px |
| Buttons | `.bg-accent:not(.rounded-full)` | 10px |
| Dialogs | `[role="dialog"].bg-surface` | 20px |
| Drawer | `aside.bg-surface` | 20px on left corners only |
| Segmented nav | `header nav.bg-surface` | full pill |

Two traps, both handled explicitly:

- **`rounded-full` would lose.** Unlayered `.bg-accent { border-radius: 10px }`
  outranks the layered `rounded-full` utility regardless of specificity, squaring
  off all 22 dots. Hence `:not(.rounded-full)`.
- **The nav pill bleeds.** Segment hover fills (`hover:bg-surface-raised`) paint
  past a rounded parent's corners, so `header nav.bg-surface` needs
  `overflow: hidden`.

The header itself stays full-bleed (a sticky edge-to-edge bar), not a floating
rounded slab — a floating toolbar would require layout changes in all 7 files for
no legibility gain.

## 5. Chrome

Today the header, drawer, dialogs and the kebab dropdown are forced **opaque**
(`--color-surface-solid`) — correct for neon, wrong here, since glass chrome is
the defining element of the macOS look. They become glass, but at a **higher
alpha than panels (~0.85)** with a stronger blur.

The reasoning that made them opaque still holds directionally: chrome floats over
content and over the photo's brightest band, so it is where text sits closest to
noise. It gets the most opacity in the system — glass, but the least transparent
glass on the page.

The kebab dropdown (`.absolute.bg-surface`) stays effectively opaque: it is small,
appears over dense list content, and has no aesthetic upside from translucency.

## 6. Typography

Uppercase wide-tracked labels are the HUD register and read as the old theme. They
soften to sentence case in the system font (`font-sans` → SF on macOS/iOS):

- Nav: `DASHBOARD` → `Dashboard`, `REMINDERS` → `Reminders`, etc.
- Drawer: `MENU` → `Menu`
- Header wordmark: `HOMELAB` → `Homelab`
- Footer: `HOMELAB` → `Homelab`
- Clock label: `LOCAL` is dropped; the time stands alone.
- Section eyebrows (`text-xs tracking-widest uppercase`, e.g. "UPCOMING") lose
  `uppercase` and `tracking-widest`.

Monospace survives only where tabular digits earn it: the header/drawer clock and
log timestamps. `tabular-nums` stays wherever it is today.

This is the only part of the change that edits the 7 HTML files and a handful of
JS strings.

## 7. Motion

**Removed:**
- `.uplink-rail` — the animated light packet under the header. Removes the
  `<div class="uplink-rail">` from all 7 pages, plus the CSS rules and the
  `uplink-sweep` keyframes.
- Gradient-filled page titles (`.page-title` + its `@supports` block). Headings
  become solid `text-ink` with a faint shadow, since `<h1>` sits directly on the
  photo with no panel behind it.

**Kept:**
- The header LED dot — recolored apricot, calmed from a throbbing
  `led-pulse` to a steady dot with a soft halo.
- `boot-rise` — the staggered panel entrance on page load.
- `dialog-enter`, `.task-clearing`, and the `.subtasks-panel` transitions, all
  unchanged.

The `prefers-reduced-motion` block stays, with the removed `.uplink-rail::after`
selector dropped from it.

## 8. Text not on a panel

Three places put text directly over the photo. Each is addressed:

- **`<h1>` page titles** — solid `text-ink` plus a faint text shadow.
- **The footer** — currently a bare `border-t` strip with no fill. It gets the
  same glass treatment as chrome so its `text-ink-dim` label stays readable.
- **Empty/loading states** (`<p class="text-sm text-ink-dim">Loading…</p>`) —
  these are always inside a `.bg-surface` panel already, so they are covered.

## 9. Non-goals

- No layout, spacing, or component-structure changes. The mobile density block
  (`--spacing: 0.2rem` below 768px) is unchanged.
- No second wallpaper for light mode. One image, two scrims.
- No changes to `api.json`, handlers, models, or any Go code.
- No JS logic changes. `lib/theme.js` keeps dark as the default and the toggle
  keeps working.

## Verification

1. `docker compose restart app` — the entrypoint recompiles Tailwind and rebuilds
   the Go binary. (Faster during iteration:
   `docker compose exec app /usr/local/bin/tailwindcss -i ./static/css/input.css -o ./static/css/style.css --minify`.)
2. Browser pass over all 7 pages (home, reminders, todos, journal, codex,
   bookmarks, logger) in **both** themes.
3. Specifically check, in this order — these are where glass legibility fails
   first:
   - small `text-ink-dim` text (timestamps, hints, metadata) on a panel over the
     photo's bright sky band;
   - accent-colored text and borders in light mode;
   - accent-button labels (`text-canvas` on `bg-accent`) in both themes;
   - the header and nav where they overlap the sky band.
4. Confirm all 22 `rounded-full` dots are still circles.
5. Mobile width (< 900px) serves the small JPEG and stays legible at
   `--spacing: 0.2rem`.

`src/app/static/css/style.css` is a gitignored build artifact — only `input.css`
is committed.

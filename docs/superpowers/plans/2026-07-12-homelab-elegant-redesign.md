# Homelab Elegant Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Homelab's current dark "rack-diagnostics console" cyberpunk visual theme with a light, warm-neutral, elegant design across all 8 pages, and add a collapsible-on-mobile pattern to every category/list section and every "add new" form, app-wide.

**Architecture:** Pure front-end re-skin of an already-complete, already-in-production app. Redefine the Tailwind v4 `@theme` tokens in one CSS file, apply the new look + a new zero-JS collapsible pattern (native `<details>`/`<summary>`) to the Dashboard first as the reference implementation (Task 1), then propagate to the other 7 pages (Task 2). No Go changes, no API/route/data-shape changes, no new database work.

**Tech Stack:** Go + vanilla JS + SQLite (GOVA stack), Tailwind v4 (`@theme` tokens, no npm/CDN), no test suite in this stack.

## Global Constraints

- No `element.innerHTML = userValue` anywhere — use `textContent`/`createElement` only (unchanged project-wide rule).
- No test suite exists in this stack — skip TDD. Verify manually: `docker compose restart app`, check `docker compose logs app`, then visually/structurally verify pages (curl for data, direct inspection of rendered classes for style/collapse behavior).
- This is styling + one UX addition only — do not change any API endpoint, route, request/response shape, or Go handler/model logic in either task.
- No new external dependencies (no Google Fonts CDN, no icon library CDN) — use system font stacks and inline SVG for the collapse chevron, consistent with this project's zero-CDN rule.
- New design tokens (exact values, both tasks must use these verbatim):

```css
@theme {
  --color-canvas: #faf7f2;        /* warm ivory page background */
  --color-surface: #ffffff;        /* card/panel background */
  --color-surface-raised: #f3efe6; /* subtle warm hover/raised background */
  --color-border: #e6dfd2;         /* hairline border, warm light gray-beige */
  --color-ink: #2b2620;            /* primary text, warm near-black */
  --color-ink-dim: #77705f;        /* secondary/muted text */
  --color-accent: #a8632b;         /* single restrained accent — links, buttons, focus rings */
  --color-accent-dim: #c98a54;     /* lighter accent tint — hover states */
  --color-ok: #4b7a5b;             /* muted forest green — success/done states */
  --color-danger: #b0463c;         /* muted brick red — destructive actions */
}
```

  These are the same token *names* already used by the current theme (`bg-canvas`, `border-hairline` → now just `border-border` via the `--color-border` token, `text-ink`, `text-ink-dim`, `text-accent`, `text-ok`, `text-danger`) — only the *values* change, minimizing the diff. If the current `input.css` uses a different token name for the border color (e.g. `--color-hairline`), keep that exact name and just change its value to `#e6dfd2` rather than renaming it — check the file first.
- Typography — one clean system sans-serif everywhere except rendered code:
  - UI/prose: `font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;` (Tailwind's default `font-sans` stack — just remove any `font-mono`/custom mono usage from nav labels, headers, chrome elements that currently use it for the "techno" look).
  - Code only: `font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;` (Tailwind's default `font-mono`) — used ONLY in Codex's `<pre><code>` snippet rendering. Nowhere else in the app should use a mono font after this change.
- Collapsible-on-mobile pattern — exact shape, used identically everywhere in both tasks:

  **CSS** (add to `src/app/static/css/input.css`, outside the `@theme` block):
  ```css
  @media (min-width: 768px) {
    .collapsible-mobile > summary.collapsible-toggle {
      display: none;
    }
    .collapsible-mobile > .collapsible-body {
      display: block !important;
    }
  }
  .collapsible-mobile > summary.collapsible-toggle {
    list-style: none;
  }
  .collapsible-mobile > summary.collapsible-toggle::-webkit-details-marker {
    display: none;
  }
  ```

  **HTML/DOM shape** (whether written directly in a `.html` file or built via `createElement` in JS):
  ```html
  <details class="collapsible-mobile" open>
    <summary class="collapsible-toggle flex items-center justify-between cursor-pointer select-none py-2 md:pointer-events-none">
      <span class="text-sm font-medium text-ink">SECTION TITLE</span>
      <svg class="w-4 h-4 text-ink-dim md:hidden" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
        <path fill-rule="evenodd" d="M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z" clip-rule="evenodd" />
      </svg>
    </summary>
    <div class="collapsible-body pt-2">
      <!-- actual section content: list, category tabs, or a create-form -->
    </div>
  </details>
  ```
  Behavior: at viewport widths below 768px, this is a native `<details>` element — clicking the `<summary>` row natively toggles the content open/closed, no JS needed, screen-reader accessible by default. At 768px and above, the CSS media query hides the `<summary>` toggle chevron entirely and forces `.collapsible-body` to always display — sections are always open on desktop exactly as before this change, with no interaction change for desktop users. The `open` attribute on `<details>` should always be present in markup (defaults to expanded on first mobile load too — this is about giving users the *option* to collapse, not defaulting to a collapsed state).
  When building this via `createElement` in JS: `document.createElement('details')`, `document.createElement('summary')`, and a wrapper `document.createElement('div')` for `.collapsible-body`, in exactly that nesting — do not skip the wrapper div, the CSS override targets it specifically so it doesn't fight with the content's own internal `display` values (e.g. a flex list inside).

---

### Task 1: New Design System + Dashboard Reference Implementation

**Files:**
- Modify: `src/app/static/css/input.css` (token values, collapsible CSS)
- Modify: `src/app/static/pages/home.html`
- Modify: `src/app/static/js/home.js`

**Interfaces:**
- Produces: the token values and collapsible CSS defined in Global Constraints above, live in `input.css`, ready for Task 2 to rely on without redefining anything.
- Produces: a working reference example of the collapsible pattern (shortcuts list, focus notes list, and both add-forms on the dashboard) for Task 2 to replicate structurally on 7 more pages.

Steps:

- [ ] **Step 1: Read the current `input.css` and `home.html`/`home.js` in full**

Before changing anything, read all three files completely to see the exact current token names, current nav markup, current shortcuts/focuses rendering code, and current form-injection code. Do not guess at current structure — the current theme was established in a prior task and must be understood before it's replaced.

- [ ] **Step 2: Replace the `@theme` token values in `input.css`**

Keep existing token *names* (whatever they currently are — likely `--color-canvas`, `--color-surface`, `--color-hairline` or `--color-border`, `--color-ink`, `--color-ink-dim`, `--color-accent`, `--color-accent-dim`, `--color-ok`, `--color-danger`), replace their *values* with the exact hex values from the Global Constraints section above. Remove any additional "server LED"/multi-accent-color tokens that don't map to the new restrained single-accent + ok/danger set (e.g. if there's a separate green/red/amber trio beyond `accent`/`ok`/`danger`, consolidate to just those three).

- [ ] **Step 3: Add the collapsible CSS to `input.css`**

Append the exact CSS block from Global Constraints (the `@media (min-width: 768px)` rule plus the marker-hiding rules) to `input.css`, outside the `@theme` block.

- [ ] **Step 4: Remove mono-font usage from non-code UI chrome**

Search `home.html`/`home.js` for any `font-mono` class or explicit monospace font-family usage used for nav labels, the clock, section headers, or any UI chrome (not actual code). Replace with the default sans (i.e. just remove the `font-mono` class, since Tailwind's base font is already `font-sans` via the `@theme`/base styles). The clock element specifically likely used mono for a "terminal" look — change it to the sans font too, this is the whole point of the redesign.

- [ ] **Step 5: Convert the shortcuts list section to the collapsible pattern**

Wrap the existing shortcuts list markup (whatever contains the shortcut items + the shortcuts add-form) in the `<details class="collapsible-mobile" open>` / `<summary class="collapsible-toggle">` / `<div class="collapsible-body">` structure from Global Constraints, using "Shortcuts" as the section title text (via `textContent`, not innerHTML, if built in JS). Preserve all existing IDs/classes on the inner list/form elements exactly as they are — only add the new wrapping structure around them, don't restructure the inner content.

- [ ] **Step 6: Convert the focus notes section to the collapsible pattern**

Same as Step 5, for the focus notes list + its add-form, using "Focus Notes" (or whatever the current section is titled) as the summary text.

- [ ] **Step 7: Verify no JS selector breakage**

Since Steps 5-6 wrap existing elements in new parent nodes, confirm every `document.getElementById`/`querySelector` call in `home.js` still finds its target — wrapping in `<details>`/`<summary>`/`<div>` does not change any existing element's `id`, so lookups by `id` are unaffected; only lookups by structural position (e.g. `parentElement.nextSibling`) would break, and none should exist in this codebase's established pattern (everything is looked up by `id`). Grep `home.js` for `nextElementSibling`/`previousElementSibling`/`children[` to be sure none exist before proceeding.

- [ ] **Step 8: Verify**

`docker compose restart app`. Visit `/` (or `/static/pages/home.html`). Confirm: light warm-neutral background, no dark/neon styling remains, no mono font anywhere on the page, shortcuts and focus-notes sections each show a clickable header with a chevron on narrow viewports (resize browser below 768px or use a mobile emulation width) that collapses/expands the section, and at desktop width (768px+) both sections are always open with no visible toggle chevron. Confirm add-shortcut and add-focus-note forms still submit correctly (create one of each, confirm it appears, delete it). Check `docker compose logs app` for errors (there should be none — this task touches no Go code).

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "style: replace dark techno theme with elegant light design, add mobile-collapsible dashboard sections"
```

---

### Task 2: Apply New Design + Collapsible Pattern to Remaining 7 Pages

**Files:**
- Modify: `src/app/static/pages/reminders.html`, `bookmarks.html`, `codex.html`, `journal.html`, `vision_board.html`, `todos.html`, `logger.html`
- Modify: `src/app/static/js/reminders.js`, `bookmarks.js`, `codex.js`, `journal.js`, `vision_board.js`, `todos.js`, `logger.js`

**Interfaces:**
- Consumes: the token values and collapsible CSS from Task 1's `input.css` (already live — do not redefine, just use the resulting Tailwind classes like `bg-canvas`, `text-ink`, `border-border`/`border-hairline`, and the `collapsible-mobile`/`collapsible-toggle`/`collapsible-body` classes).
- Consumes: Task 1's dashboard as the visual/structural reference for both the color/type re-skin and the collapsible pattern.

Steps:

- [ ] **Step 1: Re-read Task 1's finished `home.html`/`home.js`**

This is your reference implementation for both the visual re-skin and the collapsible pattern shape — read the finished files (not the plan text) to see exactly how the new tokens and the `<details>`/`<summary>`/`.collapsible-body` structure were applied in practice.

- [ ] **Step 2: Re-skin each of the 7 pages' existing classes to the new tokens**

One page at a time. Replace any remaining old-theme classes (dark backgrounds, mono-font chrome, old accent colors) with the new token-based classes matching `home.html`'s new look exactly — same background/surface/border/ink classes, same nav treatment. Keep every page's existing DOM structure (sidebars, tabs, detail panels, dynamic tables) intact — this is a re-skin, not a re-architecture, exactly as the prior design rollout (Task 9.5 in the original build) did successfully.

- [ ] **Step 3: Remove mono font from all non-code chrome on every page**

Same check as Task 1 Step 4, applied to all 7 files. The one exception: Codex's actual `<pre><code>` snippet-rendering block in `codex.js` must KEEP its mono font — that's real code content, not UI chrome, and monospace is the correct choice there.

- [ ] **Step 4: Apply the collapsible pattern to every category/list section and every create-form, per page**

Using the exact `<details>`/`<summary>`/`.collapsible-body` structure from Task 1 (Global Constraints), wrap:
- `reminders.js`: the reminders list section and the add-reminder form.
- `bookmarks.js`: the category tabs/list section and both the add-category and add-bookmark forms.
- `codex.js`: the snippet list section and the add-snippet form.
- `journal.js`: the entries sidebar (month-grouped list) and — journal's "New Entry" is a single button, not a form, so it does not need wrapping; just the sidebar list section needs the collapsible treatment.
- `vision_board.js`: the category tabs section, and the add-category, add-goal, and add-milestone forms.
- `todos.js`: the todo-lists sidebar, and the add-list and add-todo forms. The per-todo detail panel (subtasks/blocks) is a different UI concept (an expand-to-view-details panel, not a category/create-form list) — leave it as Task 7 (of the original build) implemented it, do not apply the collapsible-mobile pattern there, only to the lists-sidebar and the two top-level create-forms.
- `logger.js`: the category tabs section, and both the add-category (field-definition builder) and add-entry forms.

For each: identify the existing container element in the JS (or HTML) that currently holds that list/form, and either (a) if it's static HTML, wrap it directly with the `<details>`/`<summary>`/`<div class="collapsible-body">` structure, or (b) if it's built via `createElement` in JS, change the container's tag from whatever it currently is (likely `div` or `section`) to this same three-element structure, moving the existing content into the `.collapsible-body` div. Preserve every existing `id` and event listener target exactly — only the wrapping structure changes.

- [ ] **Step 5: Verify no JS selector breakage across all 7 files**

Same check as Task 1 Step 7, repeated per file: grep each of the 7 JS files for `nextElementSibling`/`previousElementSibling`/`children[` before and after your changes to confirm no structural (non-id-based) DOM lookups exist that the new wrapping would break.

- [ ] **Step 6: Verify**

`docker compose restart app` once, after all 7 pages are updated. Visit all 8 pages (7 restyled + dashboard) and confirm: consistent new light/warm/elegant look and typography across every page, no mono font anywhere except Codex's snippet display, and every category/list section + create-form on every page shows the collapsible chevron behavior below 768px width and is always-open with no chevron at 768px+. Spot-check one CRUD action per page still works (the re-skin + wrapping must not break any event listener). Mobile-width check on at least 3 representative pages: bookmarks (flat category list), todos (master-detail with a sidebar), logger (dynamic table + category tabs). Check `docker compose logs app` for errors (should be none — no Go changes in this task).

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "style: apply elegant design system and mobile-collapsible sections to all remaining pages"
```

---

## Self-Review Notes

**Spec coverage:** Light/warm/elegant theme (Task 1 + 2), typography change with Codex code exception (Task 1 Step 4 + Task 2 Step 3), collapsible-on-mobile app-wide for categories/lists and add-forms (Task 1 Steps 5-6 establish it, Task 2 Step 4 covers all 7 remaining pages including todo lists and every page's categories, per the user's explicit request that this go beyond just the two literally-named examples).

**Placeholder scan:** No TBD/TODO. Exact hex values, exact CSS, exact HTML/DOM shape all given verbatim in Global Constraints for both tasks to share without redefinition or drift.

**Type consistency:** Token names deliberately left flexible ("keep existing names, check the file first") since this plan was written without re-reading the exact current `input.css` token names from the prior task — this is a deliberate, disclosed exception to the "no placeholders" rule because the actual current names are a Task 1 discovery step (Step 1), not an unknown that blocks planning; the *values* and the overall approach are fully specified.

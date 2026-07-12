# App Specification

## App Name
Homelab

## What Does It Do?
A personal productivity monolith for a single user (home-network use, no public signup). It bundles a quick-launch dashboard with seven standalone tools: task management, a flexible personal logger, bookmarks, a journal, a code-snippet codex, reminders, and a vision board for long-term goals. This is a full rewrite of a legacy PHP app in the current GOVA stack, minus its AI/Discord-bot automation layer — same day-to-day features, none of the bot/automation plumbing.

## Core Features

- [ ] **Dashboard** (`/`) — quick-launch shortcuts (title + URL, user-added), a small list of "focus" notes (freeform text, manually ordered), an "upcoming reminders" widget (next 5, read-only), and a nav grid linking to the other 7 apps.

- [ ] **TaskMaster** (todos) — todo lists (ordered, renameable) containing todos (title, description, done flag, ordered via drag-reorder). Each todo can have subtasks (title, done flag) AND/OR "todo blocks" — freeform rich text sections (header + body) for longer notes attached to a todo, also ordered. Support clearing all completed todos in a list at once.

- [ ] **Logger** — user-defined log categories, where each category owner picks its own set of fields (name + type: text/date/time) at creation time and can edit that field list later. Log entries belong to a category and store one value per field defined on that category at entry-creation time. This needs a flexible/dynamic-fields data model (schema-per-category, not a fixed table) — implement as: category row stores field definitions as JSON, entry row stores values as JSON keyed by field name. List/detail UI renders columns/inputs dynamically from the category's field list.

- [ ] **Bookmarks** — categories containing bookmarks (title, URL, optional description). Auto-prepend `https://` if a submitted URL has no scheme.

- [ ] **Journal** — dated entries (title, content, mood tag: neutral/happy/great/sad/angry/tired, entry date). Sidebar lists entries grouped by month, newest first.

- [ ] **Codex** — a personal code-snippet library: entries with title, language, code body, comma-separated tags, description. Support "bundling" — linking multiple snippets together under a shared group so related snippets (e.g. a header + implementation) can be viewed together.

- [ ] **Reminders** — CRUD for reminders: title, remind-at datetime, recurrence (none/daily/weekly/monthly/specific days-of-week), active toggle. Show upcoming/overdue reminders in-app only (dashboard widget + dedicated list view). No push notifications, no external delivery — this app has no bot/automation layer.

- [ ] **Vision Board** — categories containing goals (title, target year), each goal containing milestones (title, done flag). Show live progress per goal as a percentage computed from milestone completion (not a stored flag).

## Auth
- [ ] User login required
- [ ] Public registration allowed

(No auth — single user, protected by home network access only, matching the legacy app's setup.)

## External Integrations
- [ ] Payments (Stripe)
- [ ] AI / LLM (OpenRouter)
- [ ] Other: None

(None. This app deliberately excludes the legacy Discord bot / AI newsletter / scheduled-task-automation layer.)

## Data Migration
This app replaces a legacy PHP + MySQL app with real production data (dump: `myapp_2026-07-11_11-49-01.sql`, repo root). After the schema exists (post-build), migrate data for: bookmark_categories, bookmarks, codex_entries, focuses, journal_entries, log_categories, log_entries, reminders, shortcuts, subtasks, todo_blocks, todo_lists, todos — preserving original IDs so foreign keys stay intact. Vision Board tables have no legacy data (feature existed in code but was never deployed to prod) — start empty. Excluded from migration entirely: jarvis_*, notifications, scheduled_tasks, newsletters, users, login_attempts, user_settings, events (unused/AI-automation-only tables).

## Design Notes
Full creative control delegated to the `frontend-design` skill — no specific palette/style mandated. Hard requirements: must be smooth and fully usable on both desktop and mobile (the legacy app was desktop-oriented with a cramped mobile experience — this is a chance to fix that). Follow this project's existing UI guardrails (CLAUDE.md / uncodixify-style rules already in place): no generic AI-dashboard clichés, normal/restrained radii and shadows, real information density over decorative hero sections.

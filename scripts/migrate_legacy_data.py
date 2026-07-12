#!/usr/bin/env python3
"""One-time migration: legacy MySQL dump -> Homelab SQLite app.db.

Run manually, once, after Tasks 1-9 have created the target schema in
data/app.db. This is infra tooling, not part of the running application --
it is never imported by src/app and is not wired into main.go.

Usage:
    1. Load the legacy dump into a throwaway MySQL container (see
       .superpowers/sdd/task-10-brief.md for the exact docker commands).
    2. Stop the `app` docker-compose service so nothing else holds the
       SQLite WAL file open while this script writes to it.
    3. python3 scripts/migrate_legacy_data.py
    4. Verify row counts, then restart the `app` service.
"""
import datetime
import sqlite3
import sys

import pymysql

MYSQL = dict(host="127.0.0.1", port=3307, user="root", password="migrate", database="myapp")
SQLITE_PATH = "data/app.db"

# (mysql_table, sqlite_table, [(mysql_col, sqlite_col), ...])
TABLES = [
    ("bookmark_categories", "bookmark_categories", [("id", "id"), ("title", "title"), ("created_at", "created_at")]),
    ("bookmarks", "bookmarks", [("id", "id"), ("category_id", "category_id"), ("title", "title"), ("url", "url"), ("description", "description"), ("created_at", "created_at")]),
    ("codex_entries", "codex_entries", [("id", "id"), ("title", "title"), ("language", "language"), ("code", "code"), ("tags", "tags"), ("description", "description"), ("bundle_id", "bundle_id"), ("created_at", "created_at")]),
    ("focuses", "focuses", [("id", "id"), ("text", "text"), ("sort_order", "sort_order"), ("created_at", "created_at")]),
    ("journal_entries", "journal_entries", [("id", "id"), ("title", "title"), ("content", "content"), ("mood", "mood"), ("entry_date", "entry_date"), ("created_at", "created_at")]),
    ("log_categories", "log_categories", [("id", "id"), ("title", "title"), ("schema_def", "schema_def"), ("created_at", "created_at")]),
    ("log_entries", "log_entries", [("id", "id"), ("category_id", "category_id"), ("entry_data", "entry_data"), ("created_at", "created_at")]),
    ("reminders", "reminders", [("id", "id"), ("title", "title"), ("remind_at", "remind_at"), ("recurrence_type", "recurrence_type"), ("recurrence_days", "recurrence_days"), ("is_active", "is_active"), ("created_at", "created_at")]),
    ("shortcuts", "shortcuts", [("id", "id"), ("title", "title"), ("url", "url"), ("created_at", "created_at")]),
    ("todo_lists", "todo_lists", [("id", "id"), ("title", "title"), ("sort_order", "sort_order"), ("created_at", "created_at")]),
    ("todos", "todos", [("id", "id"), ("list_id", "list_id"), ("title", "title"), ("is_done", "is_done"), ("description", "description"), ("sort_order", "sort_order"), ("created_at", "created_at")]),
    ("subtasks", "subtasks", [("id", "id"), ("todo_id", "todo_id"), ("title", "title"), ("is_done", "is_done"), ("description", "description"), ("created_at", "created_at")]),
    ("todo_blocks", "todo_blocks", [("id", "id"), ("todo_id", "todo_id"), ("header", "header"), ("content", "content"), ("sort_order", "sort_order"), ("created_at", "created_at")]),
]

# Parents must load before children so FK values already exist when children insert.
ORDER = [
    "bookmark_categories", "bookmarks",
    "codex_entries",
    "focuses",
    "journal_entries",
    "log_categories", "log_entries",
    "reminders",
    "shortcuts",
    "todo_lists", "todos", "subtasks", "todo_blocks",
]


def _normalize(value):
    """pymysql returns datetime.date/datetime objects for DATE/DATETIME/TIMESTAMP
    columns; sqlite3 (3.12+) no longer implicitly adapts these, so convert to
    ISO-format strings ourselves rather than relying on deprecated defaults."""
    if isinstance(value, (datetime.datetime, datetime.date)):
        return value.isoformat(sep=" ") if isinstance(value, datetime.datetime) else value.isoformat()
    return value


def main():
    mysql_conn = pymysql.connect(**MYSQL, cursorclass=pymysql.cursors.DictCursor)
    sqlite_conn = sqlite3.connect(SQLITE_PATH)
    sqlite_conn.execute("PRAGMA foreign_keys = OFF")  # re-enabled at the end; inserts happen parent-first anyway

    by_name = {t[0]: t for t in TABLES}

    with mysql_conn.cursor() as cur:
        for mysql_table in ORDER:
            _, sqlite_table, col_pairs = by_name[mysql_table]
            mysql_cols = [p[0] for p in col_pairs]
            sqlite_cols = [p[1] for p in col_pairs]

            cur.execute(f"SELECT {', '.join(mysql_cols)} FROM `{mysql_table}`")
            rows = cur.fetchall()

            placeholders = ", ".join("?" for _ in sqlite_cols)
            insert_sql = f"INSERT INTO {sqlite_table} ({', '.join(sqlite_cols)}) VALUES ({placeholders})"

            values = []
            for row in rows:
                values.append(tuple(_normalize(row[c]) for c in mysql_cols))

            if values:
                sqlite_conn.executemany(insert_sql, values)
            print(f"{mysql_table} -> {sqlite_table}: {len(values)} rows")

    sqlite_conn.execute("PRAGMA foreign_keys = ON")
    sqlite_conn.commit()
    sqlite_conn.close()
    mysql_conn.close()
    print("Migration complete.")


if __name__ == "__main__":
    main()

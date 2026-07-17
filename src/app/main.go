package main

import (
	"io"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"gova/app/cache"
	"gova/app/db"
	"gova/app/handlers"
	"gova/app/middleware"
	"gova/app/push"
)

func main() {
	if logPath := os.Getenv("LOG_PATH"); logPath != "" {
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
			log.SetOutput(io.MultiWriter(os.Stdout, f))
		}
	}

	if secret := os.Getenv("SESSION_SECRET"); len(secret) < 32 {
		log.Fatal("SESSION_SECRET must be set and at least 32 characters")
	}

	database, err := db.Open(os.Getenv("DB_PATH"))
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer database.Close()

	appCache := cache.New()

	if vapidPublicKey, vapidPrivateKey := os.Getenv("VAPID_PUBLIC_KEY"), os.Getenv("VAPID_PRIVATE_KEY"); vapidPublicKey == "" || vapidPrivateKey == "" {
		log.Println("VAPID_PUBLIC_KEY/VAPID_PRIVATE_KEY not set — push notifications disabled")
	} else {
		subscriber := os.Getenv("VAPID_SUBSCRIBER")
		if subscriber == "" {
			subscriber = "mailto:admin@localhost"
		}
		push.Start(database.Read, database.Write, appCache, vapidPublicKey, vapidPrivateKey, subscriber)
	}

	r := chi.NewRouter()
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.Security)
	r.Use(middleware.CSRF)
	r.Use(middleware.Auth)

	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	r.Get("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/sw.js")
	})

	// Pages
	r.Get("/", handlers.HomeGET())

	r.Get("/api/reminders", handlers.RemindersGET(database.Read, database.Write, appCache))
	r.Post("/api/reminders_create", handlers.RemindersCreatePOST(database.Read, database.Write, appCache))
	r.Put("/api/reminders/{id}", handlers.RemindersUpdatePUT(database.Read, database.Write, appCache))
	r.Delete("/api/reminders/{id}", handlers.RemindersDeleteDELETE(database.Read, database.Write, appCache))
	r.Post("/api/reminders/{id}/toggle", handlers.RemindersTogglePOST(database.Read, database.Write, appCache))

	r.Post("/api/push_subscribe", handlers.PushSubscribePOST(database.Read, database.Write, appCache))
	r.Get("/api/push_public_key", handlers.PushPublicKeyGET())

	r.Get("/api/bookmarks", handlers.BookmarksGET(database.Read, database.Write, appCache))
	r.Post("/api/bookmark_categories_create", handlers.BookmarkCategoriesCreatePOST(database.Read, database.Write, appCache))
	r.Put("/api/bookmark_categories/{id}", handlers.BookmarkCategoriesUpdatePUT(database.Read, database.Write, appCache))
	r.Delete("/api/bookmark_categories/{id}", handlers.BookmarkCategoriesDeleteDELETE(database.Read, database.Write, appCache))
	r.Post("/api/bookmarks_create", handlers.BookmarksCreatePOST(database.Read, database.Write, appCache))
	r.Put("/api/bookmarks/{id}", handlers.BookmarksUpdatePUT(database.Read, database.Write, appCache))
	r.Delete("/api/bookmarks/{id}", handlers.BookmarksDeleteDELETE(database.Read, database.Write, appCache))

	r.Get("/api/codex", handlers.CodexGET(database.Read, database.Write, appCache))
	r.Post("/api/codex_entries_create", handlers.CodexEntriesCreatePOST(database.Read, database.Write, appCache))
	r.Put("/api/codex_entries/{id}", handlers.CodexEntriesUpdatePUT(database.Read, database.Write, appCache))
	r.Delete("/api/codex_entries/{id}", handlers.CodexEntriesDeleteDELETE(database.Read, database.Write, appCache))
	r.Post("/api/codex_folders_rename", handlers.CodexFoldersRenamePOST(database.Read, database.Write, appCache))
	r.Post("/api/codex_folders_delete", handlers.CodexFoldersDeletePOST(database.Read, database.Write, appCache))

	r.Get("/api/journal", handlers.JournalGET(database.Read, database.Write, appCache))
	r.Post("/api/journal_entries_create", handlers.JournalEntriesCreatePOST(database.Read, database.Write, appCache))
	r.Put("/api/journal_entries/{id}", handlers.JournalEntriesUpdatePUT(database.Read, database.Write, appCache))
	r.Delete("/api/journal_entries/{id}", handlers.JournalEntriesDeleteDELETE(database.Read, database.Write, appCache))

	r.Get("/api/todos", handlers.TodosGET(database.Read, database.Write, appCache))
	r.Post("/api/todo_lists_create", handlers.TodoListsCreatePOST(database.Read, database.Write, appCache))
	r.Put("/api/todo_lists/{id}", handlers.TodoListsUpdatePUT(database.Read, database.Write, appCache))
	r.Delete("/api/todo_lists/{id}", handlers.TodoListsDeleteDELETE(database.Read, database.Write, appCache))
	r.Post("/api/todo_lists/{id}/clear_completed", handlers.TodosClearCompletedPOST(database.Read, database.Write, appCache))
	r.Post("/api/todos_create", handlers.TodosCreatePOST(database.Read, database.Write, appCache))
	r.Put("/api/todos/{id}", handlers.TodosUpdatePUT(database.Read, database.Write, appCache))
	r.Post("/api/todos/{id}/toggle", handlers.TodosTogglePOST(database.Read, database.Write, appCache))
	r.Delete("/api/todos/{id}", handlers.TodosDeleteDELETE(database.Read, database.Write, appCache))
	r.Put("/api/todos_reorder", handlers.TodosReorderPUT(database.Read, database.Write, appCache))

	r.Get("/api/todos/{id}/details", handlers.TodoDetailsGET(database.Read, database.Write, appCache))
	r.Post("/api/subtasks_create", handlers.SubtasksCreatePOST(database.Read, database.Write, appCache))
	r.Post("/api/subtasks/{id}/toggle", handlers.SubtasksTogglePOST(database.Read, database.Write, appCache))
	r.Put("/api/subtasks/{id}", handlers.SubtasksUpdatePUT(database.Read, database.Write, appCache))
	r.Delete("/api/subtasks/{id}", handlers.SubtasksDeleteDELETE(database.Read, database.Write, appCache))
	r.Post("/api/todo_blocks_create", handlers.TodoBlocksCreatePOST(database.Read, database.Write, appCache))
	r.Put("/api/todo_blocks/{id}", handlers.TodoBlocksUpdatePUT(database.Read, database.Write, appCache))
	r.Delete("/api/todo_blocks/{id}", handlers.TodoBlocksDeleteDELETE(database.Read, database.Write, appCache))

	r.Get("/api/logger", handlers.LoggerGET(database.Read, database.Write, appCache))
	r.Get("/api/log_categories/{id}/entries", handlers.LogEntriesByCategoryGET(database.Read, database.Write, appCache))
	r.Post("/api/log_categories_create", handlers.LogCategoriesCreatePOST(database.Read, database.Write, appCache))
	r.Put("/api/log_categories/{id}", handlers.LogCategoriesUpdatePUT(database.Read, database.Write, appCache))
	r.Delete("/api/log_categories/{id}", handlers.LogCategoriesDeleteDELETE(database.Read, database.Write, appCache))
	r.Post("/api/log_entries_create", handlers.LogEntriesCreatePOST(database.Read, database.Write, appCache))
	r.Delete("/api/log_entries/{id}", handlers.LogEntriesDeleteDELETE(database.Read, database.Write, appCache))

	r.Get("/api/dashboard", handlers.DashboardGET(database.Read, database.Write, appCache))
	r.Post("/api/shortcuts_create", handlers.ShortcutsCreatePOST(database.Read, database.Write, appCache))
	r.Delete("/api/shortcuts/{id}", handlers.ShortcutsDeleteDELETE(database.Read, database.Write, appCache))
	r.Put("/api/shortcuts_reorder", handlers.ShortcutsReorderPUT(database.Read, database.Write, appCache))
	r.Post("/api/focuses_create", handlers.FocusesCreatePOST(database.Read, database.Write, appCache))
	r.Delete("/api/focuses/{id}", handlers.FocusesDeleteDELETE(database.Read, database.Write, appCache))

	// Generated API routes registered here by MCP tools
	// Use database.Read for GET handlers, database.Write for POST handlers
	// Example:
	//   r.Post("/api/auth/login",  handlers.LoginPOST(database.Read, database.Write, appCache))
	//   r.Post("/api/auth/logout", handlers.LogoutPOST())
	//   r.Get("/api/auth/me",      handlers.MeGET(database.Read, database.Write, appCache))

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("GOVA app listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

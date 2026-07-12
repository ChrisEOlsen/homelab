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
	_ = appCache

	r := chi.NewRouter()
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)
	r.Use(middleware.Security)
	r.Use(middleware.CSRF)
	r.Use(middleware.Auth)

	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// Pages
	r.Get("/", handlers.HomeGET())

	r.Get("/api/reminders", handlers.RemindersGET(database.Read, database.Write, appCache))
	r.Post("/api/reminders_create", handlers.RemindersCreatePOST(database.Read, database.Write, appCache))
	r.Put("/api/reminders/{id}", handlers.RemindersUpdatePUT(database.Read, database.Write, appCache))
	r.Delete("/api/reminders/{id}", handlers.RemindersDeleteDELETE(database.Read, database.Write, appCache))
	r.Post("/api/reminders/{id}/toggle", handlers.RemindersTogglePOST(database.Read, database.Write, appCache))

	r.Get("/api/bookmarks", handlers.BookmarksGET(database.Read, database.Write, appCache))
	r.Post("/api/bookmark_categories_create", handlers.BookmarkCategoriesCreatePOST(database.Read, database.Write, appCache))
	r.Delete("/api/bookmark_categories/{id}", handlers.BookmarkCategoriesDeleteDELETE(database.Read, database.Write, appCache))
	r.Post("/api/bookmarks_create", handlers.BookmarksCreatePOST(database.Read, database.Write, appCache))
	r.Put("/api/bookmarks/{id}", handlers.BookmarksUpdatePUT(database.Read, database.Write, appCache))
	r.Delete("/api/bookmarks/{id}", handlers.BookmarksDeleteDELETE(database.Read, database.Write, appCache))

	r.Get("/api/codex", handlers.CodexGET(database.Read, database.Write, appCache))
	r.Post("/api/codex_entries_create", handlers.CodexEntriesCreatePOST(database.Read, database.Write, appCache))
	r.Put("/api/codex_entries/{id}", handlers.CodexEntriesUpdatePUT(database.Read, database.Write, appCache))
	r.Delete("/api/codex_entries/{id}", handlers.CodexEntriesDeleteDELETE(database.Read, database.Write, appCache))

	r.Get("/api/journal", handlers.JournalGET(database.Read, database.Write, appCache))
	r.Post("/api/journal_entries_create", handlers.JournalEntriesCreatePOST(database.Read, database.Write, appCache))
	r.Put("/api/journal_entries/{id}", handlers.JournalEntriesUpdatePUT(database.Read, database.Write, appCache))
	r.Delete("/api/journal_entries/{id}", handlers.JournalEntriesDeleteDELETE(database.Read, database.Write, appCache))

	r.Get("/api/vision_board", handlers.VisionBoardGET(database.Read, database.Write, appCache))
	r.Post("/api/vision_categories_create", handlers.VisionCategoriesCreatePOST(database.Read, database.Write, appCache))
	r.Delete("/api/vision_categories/{id}", handlers.VisionCategoriesDeleteDELETE(database.Read, database.Write, appCache))
	r.Post("/api/vision_goals_create", handlers.VisionGoalsCreatePOST(database.Read, database.Write, appCache))
	r.Delete("/api/vision_goals/{id}", handlers.VisionGoalsDeleteDELETE(database.Read, database.Write, appCache))
	r.Post("/api/vision_milestones_create", handlers.VisionMilestonesCreatePOST(database.Read, database.Write, appCache))
	r.Post("/api/vision_milestones/{id}/toggle", handlers.VisionMilestonesTogglePOST(database.Read, database.Write, appCache))
	r.Delete("/api/vision_milestones/{id}", handlers.VisionMilestonesDeleteDELETE(database.Read, database.Write, appCache))

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

package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"gova/app/cache"
	"gova/app/calendar"
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

	// Fallbacks for a path that matched no route, and for a method that is not
	// registered on a path that did. chi's defaults answer in plain text, which
	// breaks the envelope contract for /api/ callers; these answer in the
	// envelope there and leave human-facing URLs alone.
	r.NotFound(handlers.NotFoundHandler())
	r.MethodNotAllowed(handlers.MethodNotAllowedHandler())

	// Static files
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// Pages. Source of truth: api.json's "pages" -> handlers/pages_gen.go.
	// Never hand-wire a page route here; create_page and the scaffold tools
	// regenerate RegisterPages. "/" is the framework's own home shell and is
	// not in the manifest.
	r.Get("/", handlers.HomeGET())
	handlers.RegisterPages(r)

	// Framework API endpoints (not scaffolded — wired directly).
	r.Get("/api/v1/_version", handlers.VersionGET())
	r.Get("/api/v1/_manifest", handlers.ManifestGET())

	// Generated API routes. Source of truth: api.json -> handlers/routes_gen.go.
	// Never hand-edit routes here; scaffold tools regenerate RegisterGenerated.
	handlers.RegisterGenerated(r, database, appCache)

	// Background calendar sync. Runs once at boot and then on an interval;
	// CALENDAR_SYNC_INTERVAL_MIN=0 disables it entirely and leaves the page's
	// Sync button as the only trigger.
	if mins := envInt("CALENDAR_SYNC_INTERVAL_MIN", 30); mins > 0 {
		go func() {
			svc := calendar.NewFromDB(database.Read, database.Write, appCache)
			run := func() {
				res := svc.Run(context.Background())
				log.Printf("calendar sync: ok=%v seen=%d created=%d updated=%d cancelled=%d err=%q",
					res.OK, res.EventsSeen, res.Created, res.Updated, res.Cancelled, res.Error)
			}
			run()
			ticker := time.NewTicker(time.Duration(mins) * time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				run()
			}
		}()
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("GOVA app listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

// envInt reads an integer environment variable, falling back to def when unset
// or unparseable.
func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

// Command server runs Pinalert's HTTP API. It never applies database
// migrations itself — see cmd/migrate — and it never generates a random
// SESSION_SECRET: on a free hosting tier that sleeps and cold-starts
// frequently, doing so would silently invalidate every visitor's session on
// every restart, and once Phase 2 makes sessions load-bearing for
// reputation, would destroy that continuity outright.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"pinalert/internal/api"
	"pinalert/internal/service"
	"pinalert/internal/session"
	"pinalert/internal/store"
	sqlcgen "pinalert/internal/store/sqlc"
)

const devInsecureSecret = "dev-only-insecure-secret-do-not-use-in-production"

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL must be set")
	}

	env := os.Getenv("ENV")
	secret := loadSessionSecret(env)

	ctx := context.Background()
	pool, err := store.NewPool(ctx, dsn)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer pool.Close()

	sessionMgr, err := session.NewManager([]byte(secret), env != "development")
	if err != nil {
		log.Fatalf("constructing session manager: %v", err)
	}

	queries := sqlcgen.New(pool)
	deps := api.Deps{
		Session:  sessionMgr,
		Sessions: queries,
		Reports:  service.NewReportService(queries),
	}
	router := api.NewRouter(deps)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("pinalert listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

// loadSessionSecret reads SESSION_SECRET, failing fast unless env is
// "development" — the process must never auto-generate a random secret at
// startup (see package doc comment above).
func loadSessionSecret(env string) string {
	secret := os.Getenv("SESSION_SECRET")
	if secret != "" {
		return secret
	}
	if env != "development" {
		log.Fatal("SESSION_SECRET must be set outside ENV=development")
	}
	log.Println("WARNING: SESSION_SECRET unset; using insecure development fallback secret")
	return devInsecureSecret
}

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
	"strconv"
	"time"

	"pinalert/internal/api"
	"pinalert/internal/api/handlers"
	"pinalert/internal/mailer"
	"pinalert/internal/service"
	"pinalert/internal/session"
	"pinalert/internal/store"
	sqlcgen "pinalert/internal/store/sqlc"
)

const devInsecureSecret = "dev-only-insecure-secret-do-not-use-in-production"

// Fallback map centre for visitors who deny geolocation (Bengaluru), and the
// default nearby-reports search radius. Configurable here rather than
// hard-coded in JS — see handlers.PageConfig.
const (
	fallbackLat     = 12.9716
	fallbackLon     = 77.5946
	defaultRadiusKm = 10.0
)

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

	tmpl, err := handlers.ParsePageTemplate()
	if err != nil {
		log.Fatalf("parsing page template: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// BASE_URL roots the absolute verification links AuthService mints
	// (e.g. https://pinalert.example/auth/verify?token=...) — it must be the
	// externally reachable origin, not necessarily this process's own bind
	// address, so it defaults to a same-host guess only for local
	// development and should always be set explicitly in any real
	// deployment.
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:" + port
	}

	// The log-only mailer is the only implementation this plan wires up;
	// plan 01.1-03 adds a Resend-backed implementation selected via
	// RESEND_API_KEY, following the same fail-fast-outside-development
	// pattern loadSessionSecret already establishes above.
	mail := mailer.NewLogMailer()

	assetVersion := strconv.FormatInt(time.Now().Unix(), 10)

	queries := sqlcgen.New(pool)
	deps := api.Deps{
		Session:     sessionMgr,
		Sessions:    queries,
		Reports:     service.NewReportService(queries),
		AuthService: service.NewAuthService(queries, mail, baseURL),
		Auth: handlers.AuthConfig{
			AssetVersion: assetVersion,
		},
		Template: tmpl,
		Page: handlers.PageConfig{
			FallbackLat:     fallbackLat,
			FallbackLon:     fallbackLon,
			DefaultRadiusKm: defaultRadiusKm,
			AssetVersion:    assetVersion,
		},
		Dev: env == "development",
	}
	router := api.NewRouter(deps)

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

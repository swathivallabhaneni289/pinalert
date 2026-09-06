// Command migrate is the only thing that applies Pinalert's schema. It is a
// standalone binary, deliberately separate from cmd/server: migrations must
// never run on application boot, because a platform that ever runs more than
// one instance would race two concurrent migration runs.
//
// Usage:
//
//	DATABASE_URL=postgres://... go run ./cmd/migrate         # applies all pending migrations (goose up)
//	DATABASE_URL=postgres://... go run ./cmd/migrate down    # rolls back the most recent migration
//	DATABASE_URL=postgres://... go run ./cmd/migrate status  # prints migration status, applies nothing
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
	"github.com/pressly/goose/v3"

	"pinalert/internal/store"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL must be set (e.g. postgres://localhost:5432/pinalert?sslmode=disable)")
		os.Exit(1)
	}

	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("opening database connection: %v", err)
	}
	defer db.Close()

	goose.SetBaseFS(store.MigrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatalf("setting goose dialect: %v", err)
	}

	switch command {
	case "up":
		if err := goose.Up(db, "migrations"); err != nil {
			log.Fatalf("applying migrations: %v", err)
		}
	case "down":
		if err := goose.Down(db, "migrations"); err != nil {
			log.Fatalf("rolling back migration: %v", err)
		}
	case "status":
		if err := goose.Status(db, "migrations"); err != nil {
			log.Fatalf("checking migration status: %v", err)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q; expected up, down, or status\n", command)
		os.Exit(1)
	}
}

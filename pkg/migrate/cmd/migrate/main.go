// Command migrate is a reference implementation showing how services
// wire their migrations into an executable binary.
//
// Each service should create its own cmd/migrate/main.go that imports
// the service's migration definitions and calls migrate.RunCLI().
//
// Build: go build -o migrate ./cmd/migrate
// Run:   DATABASE_URL=postgres://... ./migrate up
//        DATABASE_URL=postgres://... ./migrate down 1
//        DATABASE_URL=postgres://... ./migrate status
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/auction-system/pkg/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

// migrations defines the schema changes for the service.
// Every migration MUST have both Up and Down operations (Requirement 7.2).
var migrations = []migrate.Migration{
	{
		Version: 20240101000001,
		Name:    "create_schema_migrations",
		Up: func(ctx context.Context, pool *pgxpool.Pool) error {
			// This migration is a no-op since the runner creates
			// the schema_migrations table automatically.
			// It serves as the baseline migration.
			return nil
		},
		Down: func(ctx context.Context, pool *pgxpool.Pool) error {
			// Dropping schema_migrations is dangerous in production.
			// This is a baseline migration — down is a no-op.
			return nil
		},
	},
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "Usage: migrate <up|down|status> [steps]\n")
		fmt.Fprintf(os.Stderr, "\nCommands:\n")
		fmt.Fprintf(os.Stderr, "  up       Apply all pending migrations\n")
		fmt.Fprintf(os.Stderr, "  down N   Rollback last N migrations (default: 1)\n")
		fmt.Fprintf(os.Stderr, "  status   Show migration status\n")
		fmt.Fprintf(os.Stderr, "\nEnvironment:\n")
		fmt.Fprintf(os.Stderr, "  DATABASE_URL       PostgreSQL connection string (required)\n")
		fmt.Fprintf(os.Stderr, "  MIGRATION_TIMEOUT  Overall timeout in seconds (default: 300)\n")
		fmt.Fprintf(os.Stderr, "  ROLLBACK_TIMEOUT   Rollback timeout in seconds (default: 120)\n")
		os.Exit(1)
	}

	migrate.RunCLI(migrations, args...)
}

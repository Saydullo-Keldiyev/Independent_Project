package migrate

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// RunCLI provides a command-line interface for running migrations.
// It reads configuration from environment variables and runs the
// specified direction (up/down).
//
// Environment variables:
//   - DATABASE_URL: PostgreSQL connection string (required)
//   - MIGRATION_TIMEOUT: Overall timeout in seconds (default: 300)
//   - ROLLBACK_TIMEOUT: Rollback timeout in seconds (default: 120)
//
// Usage:
//
//	RunCLI(migrations, "up")      — apply all pending migrations
//	RunCLI(migrations, "down")    — rollback last migration
//	RunCLI(migrations, "down", 3) — rollback last 3 migrations
//	RunCLI(migrations, "status")  — print migration status
func RunCLI(migrations []Migration, args ...string) {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	if len(args) == 0 {
		logger.Fatal("usage: migrate <up|down|status> [steps]")
	}

	direction := args[0]

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		logger.Fatal("DATABASE_URL environment variable is required")
	}

	timeout := parseDurationEnv("MIGRATION_TIMEOUT", 300)
	rollbackTimeout := parseDurationEnv("ROLLBACK_TIMEOUT", 120)

	// Connect to database
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	config := Config{
		Timeout:         timeout,
		RollbackTimeout: rollbackTimeout,
		Logger:          logger,
	}

	runner := NewRunner(pool, migrations, config)

	switch direction {
	case "up":
		result := runner.Up(context.Background())
		reportResult(logger, result, DirectionUp)
		if result.Error != nil {
			os.Exit(1)
		}

	case "down":
		steps := 1
		if len(args) > 1 {
			s, err := strconv.Atoi(args[1])
			if err != nil {
				logger.Fatal("invalid steps argument", zap.String("steps", args[1]))
			}
			steps = s
		}
		result := runner.Down(context.Background(), steps)
		reportResult(logger, result, DirectionDown)
		if result.Error != nil {
			os.Exit(1)
		}

	case "status":
		statuses, err := runner.Status(context.Background())
		if err != nil {
			logger.Fatal("failed to get migration status", zap.Error(err))
		}
		printStatus(statuses)

	default:
		logger.Fatal("unknown direction", zap.String("direction", direction))
	}
}

// reportResult logs the outcome of a migration run.
func reportResult(logger *zap.Logger, result *Result, direction Direction) {
	if result.Error != nil {
		logger.Error("migration run failed",
			zap.String("direction", string(direction)),
			zap.String("failed_migration", result.FailedMigration),
			zap.Error(result.Error),
			zap.Int("applied_count", len(result.Applied)),
			zap.Int("rolled_back_count", len(result.RolledBack)),
			zap.Duration("duration", result.Duration),
		)
	} else {
		logger.Info("migration run completed successfully",
			zap.String("direction", string(direction)),
			zap.Int("applied_count", len(result.Applied)),
			zap.Int("rolled_back_count", len(result.RolledBack)),
			zap.Duration("duration", result.Duration),
		)
	}
}

// printStatus outputs the current migration status to stdout.
func printStatus(statuses []MigrationStatus) {
	fmt.Printf("%-15s %-40s %-10s %-10s %s\n", "VERSION", "NAME", "APPLIED", "DIRTY", "APPLIED_AT")
	fmt.Println("--------------------------------------------------------------------------------------------")
	for _, s := range statuses {
		appliedStr := "no"
		if s.Applied {
			appliedStr = "yes"
		}
		dirtyStr := ""
		if s.Dirty {
			dirtyStr = "DIRTY"
		}
		appliedAt := ""
		if !s.AppliedAt.IsZero() {
			appliedAt = s.AppliedAt.Format(time.RFC3339)
		}
		fmt.Printf("%-15d %-40s %-10s %-10s %s\n", s.Version, s.Name, appliedStr, dirtyStr, appliedAt)
	}
}

// parseDurationEnv reads a seconds value from an environment variable.
func parseDurationEnv(key string, defaultSeconds int) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return time.Duration(defaultSeconds) * time.Second
	}
	seconds, err := strconv.Atoi(val)
	if err != nil {
		return time.Duration(defaultSeconds) * time.Second
	}
	return time.Duration(seconds) * time.Second
}

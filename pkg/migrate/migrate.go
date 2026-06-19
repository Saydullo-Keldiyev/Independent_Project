// Package migrate provides a database migration framework with rollback support.
// It tracks applied migrations in a schema_migrations table and supports
// both forward ("up") and reverse ("down") operations.
//
// Requirements: 7.1, 7.2, 7.5
package migrate

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Direction indicates whether a migration runs forward or in reverse.
type Direction string

const (
	DirectionUp   Direction = "up"
	DirectionDown Direction = "down"
)

// Migration represents a single schema migration with forward and reverse operations.
type Migration struct {
	// Version is a unique monotonically increasing identifier (e.g., Unix timestamp).
	Version int64
	// Name is a human-readable description of the migration.
	Name string
	// Up executes the forward migration.
	Up func(ctx context.Context, pool *pgxpool.Pool) error
	// Down executes the reverse migration (rollback).
	Down func(ctx context.Context, pool *pgxpool.Pool) error
}

// Config holds migration runner configuration.
type Config struct {
	// Timeout is the maximum duration for the entire migration run (default: 300s per Req 7.1).
	Timeout time.Duration
	// RollbackTimeout is the max duration for rollback on failure (default: 120s per Req 7.5).
	RollbackTimeout time.Duration
	// Logger is the structured logger instance.
	Logger *zap.Logger
}

// DefaultConfig returns a Config with production defaults matching requirements.
func DefaultConfig(logger *zap.Logger) Config {
	return Config{
		Timeout:         300 * time.Second,
		RollbackTimeout: 120 * time.Second,
		Logger:          logger,
	}
}

// Result holds the outcome of a migration run.
type Result struct {
	// Applied is the list of migration versions that were successfully applied.
	Applied []int64
	// RolledBack is the list of migration versions that were rolled back.
	RolledBack []int64
	// Error is non-nil if the migration failed.
	Error error
	// FailedMigration is the name of the migration file that caused the error.
	FailedMigration string
	// Duration is how long the migration run took.
	Duration time.Duration
}

// Runner executes database migrations.
type Runner struct {
	pool       *pgxpool.Pool
	migrations []Migration
	config     Config
}

// NewRunner creates a new migration runner.
func NewRunner(pool *pgxpool.Pool, migrations []Migration, config Config) *Runner {
	if config.Logger == nil {
		config.Logger = zap.NewNop()
	}
	if config.Timeout == 0 {
		config.Timeout = 300 * time.Second
	}
	if config.RollbackTimeout == 0 {
		config.RollbackTimeout = 120 * time.Second
	}

	// Sort migrations by version ascending
	sorted := make([]Migration, len(migrations))
	copy(sorted, migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version < sorted[j].Version
	})

	return &Runner{
		pool:       pool,
		migrations: sorted,
		config:     config,
	}
}

// EnsureTable creates the schema_migrations tracking table if it does not exist.
func (r *Runner) EnsureTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     BIGINT PRIMARY KEY,
			dirty       BOOLEAN NOT NULL DEFAULT false,
			applied_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`
	_, err := r.pool.Exec(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}
	return nil
}

// Up runs all pending migrations. On failure, it rolls back the failed migration
// and reports the failure with the migration file name.
func (r *Runner) Up(ctx context.Context) *Result {
	start := time.Now()
	result := &Result{}

	// Apply overall timeout (Requirement 7.1: 300s)
	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	// Ensure tracking table exists
	if err := r.EnsureTable(ctx); err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result
	}

	// Get already applied versions
	applied, err := r.getAppliedVersions(ctx)
	if err != nil {
		result.Error = fmt.Errorf("failed to get applied versions: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	appliedSet := make(map[int64]bool, len(applied))
	for _, v := range applied {
		appliedSet[v] = true
	}

	// Run pending migrations in order
	for _, m := range r.migrations {
		if appliedSet[m.Version] {
			continue
		}

		// Check context deadline
		if ctx.Err() != nil {
			result.Error = fmt.Errorf("migration timeout exceeded: %w", ctx.Err())
			result.FailedMigration = migrationName(m)
			result.Duration = time.Since(start)
			r.config.Logger.Error("migration timeout",
				zap.String("migration", migrationName(m)),
				zap.Duration("elapsed", time.Since(start)),
			)
			// Attempt rollback of already-applied migrations in this run
			r.rollbackApplied(context.Background(), result)
			return result
		}

		r.config.Logger.Info("applying migration",
			zap.Int64("version", m.Version),
			zap.String("name", m.Name),
			zap.String("direction", string(DirectionUp)),
		)

		// Mark as dirty before execution
		if err := r.markDirty(ctx, m.Version); err != nil {
			result.Error = fmt.Errorf("failed to mark migration %s dirty: %w", migrationName(m), err)
			result.FailedMigration = migrationName(m)
			result.Duration = time.Since(start)
			return result
		}

		// Execute the up migration
		if err := m.Up(ctx, r.pool); err != nil {
			result.Error = fmt.Errorf("migration %s failed: %w", migrationName(m), err)
			result.FailedMigration = migrationName(m)

			r.config.Logger.Error("migration failed",
				zap.Int64("version", m.Version),
				zap.String("name", m.Name),
				zap.Error(err),
			)

			// Remove dirty marker
			_ = r.removeMigrationRecord(ctx, m.Version)

			// Rollback previously applied migrations in this run (Requirement 7.5)
			r.rollbackApplied(context.Background(), result)
			result.Duration = time.Since(start)
			return result
		}

		// Mark as clean (applied successfully)
		if err := r.markClean(ctx, m.Version); err != nil {
			result.Error = fmt.Errorf("failed to mark migration %s clean: %w", migrationName(m), err)
			result.FailedMigration = migrationName(m)
			result.Duration = time.Since(start)
			return result
		}

		result.Applied = append(result.Applied, m.Version)
		r.config.Logger.Info("migration applied successfully",
			zap.Int64("version", m.Version),
			zap.String("name", m.Name),
		)
	}

	result.Duration = time.Since(start)
	return result
}

// Down rolls back the last N applied migrations in reverse order.
func (r *Runner) Down(ctx context.Context, steps int) *Result {
	start := time.Now()
	result := &Result{}

	// Apply rollback timeout (Requirement 7.5: 120s)
	ctx, cancel := context.WithTimeout(ctx, r.config.RollbackTimeout)
	defer cancel()

	// Ensure tracking table exists
	if err := r.EnsureTable(ctx); err != nil {
		result.Error = err
		result.Duration = time.Since(start)
		return result
	}

	// Get applied versions in descending order
	applied, err := r.getAppliedVersions(ctx)
	if err != nil {
		result.Error = fmt.Errorf("failed to get applied versions: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Sort descending for rollback
	sort.Slice(applied, func(i, j int) bool {
		return applied[i] > applied[j]
	})

	// Build migration lookup map
	migrationMap := make(map[int64]Migration, len(r.migrations))
	for _, m := range r.migrations {
		migrationMap[m.Version] = m
	}

	// Rollback up to N steps
	count := 0
	for _, version := range applied {
		if count >= steps {
			break
		}

		m, ok := migrationMap[version]
		if !ok {
			result.Error = fmt.Errorf("migration version %d not found in registered migrations", version)
			result.Duration = time.Since(start)
			return result
		}

		if m.Down == nil {
			result.Error = fmt.Errorf("migration %s has no down operation", migrationName(m))
			result.FailedMigration = migrationName(m)
			result.Duration = time.Since(start)
			return result
		}

		r.config.Logger.Info("rolling back migration",
			zap.Int64("version", m.Version),
			zap.String("name", m.Name),
			zap.String("direction", string(DirectionDown)),
		)

		if err := m.Down(ctx, r.pool); err != nil {
			result.Error = fmt.Errorf("rollback of %s failed: %w", migrationName(m), err)
			result.FailedMigration = migrationName(m)
			result.Duration = time.Since(start)

			r.config.Logger.Error("rollback failed",
				zap.Int64("version", m.Version),
				zap.String("name", m.Name),
				zap.Error(err),
			)
			return result
		}

		// Remove migration record
		if err := r.removeMigrationRecord(ctx, version); err != nil {
			result.Error = fmt.Errorf("failed to remove migration record %d: %w", version, err)
			result.Duration = time.Since(start)
			return result
		}

		result.RolledBack = append(result.RolledBack, version)
		count++

		r.config.Logger.Info("migration rolled back successfully",
			zap.Int64("version", m.Version),
			zap.String("name", m.Name),
		)
	}

	result.Duration = time.Since(start)
	return result
}

// Status returns the current migration status.
func (r *Runner) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := r.EnsureTable(ctx); err != nil {
		return nil, err
	}

	applied, err := r.getAppliedVersionsWithDetails(ctx)
	if err != nil {
		return nil, err
	}

	appliedMap := make(map[int64]MigrationStatus, len(applied))
	for _, s := range applied {
		appliedMap[s.Version] = s
	}

	var statuses []MigrationStatus
	for _, m := range r.migrations {
		if s, ok := appliedMap[m.Version]; ok {
			s.Name = m.Name
			statuses = append(statuses, s)
		} else {
			statuses = append(statuses, MigrationStatus{
				Version: m.Version,
				Name:    m.Name,
				Applied: false,
			})
		}
	}

	return statuses, nil
}

// MigrationStatus represents the status of a single migration.
type MigrationStatus struct {
	Version   int64
	Name      string
	Applied   bool
	Dirty     bool
	AppliedAt time.Time
}

// rollbackApplied rolls back migrations that were applied during this run.
func (r *Runner) rollbackApplied(ctx context.Context, result *Result) {
	if len(result.Applied) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, r.config.RollbackTimeout)
	defer cancel()

	r.config.Logger.Warn("initiating rollback of applied migrations",
		zap.Int("count", len(result.Applied)),
	)

	// Build migration lookup
	migrationMap := make(map[int64]Migration, len(r.migrations))
	for _, m := range r.migrations {
		migrationMap[m.Version] = m
	}

	// Rollback in reverse order
	for i := len(result.Applied) - 1; i >= 0; i-- {
		version := result.Applied[i]
		m, ok := migrationMap[version]
		if !ok {
			r.config.Logger.Error("cannot rollback: migration not found",
				zap.Int64("version", version),
			)
			continue
		}

		if m.Down == nil {
			r.config.Logger.Error("cannot rollback: no down operation",
				zap.Int64("version", version),
				zap.String("name", m.Name),
			)
			continue
		}

		if err := m.Down(ctx, r.pool); err != nil {
			r.config.Logger.Error("rollback failed",
				zap.Int64("version", version),
				zap.String("name", m.Name),
				zap.Error(err),
			)
			continue
		}

		_ = r.removeMigrationRecord(ctx, version)
		result.RolledBack = append(result.RolledBack, version)

		r.config.Logger.Info("rollback completed",
			zap.Int64("version", version),
			zap.String("name", m.Name),
		)
	}
}

// getAppliedVersions returns all applied migration versions sorted ascending.
func (r *Runner) getAppliedVersions(ctx context.Context) ([]int64, error) {
	rows, err := r.pool.Query(ctx, "SELECT version FROM schema_migrations WHERE dirty = false ORDER BY version ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []int64
	for rows.Next() {
		var v int64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}

// getAppliedVersionsWithDetails returns detailed status for all applied migrations.
func (r *Runner) getAppliedVersionsWithDetails(ctx context.Context) ([]MigrationStatus, error) {
	rows, err := r.pool.Query(ctx,
		"SELECT version, dirty, applied_at FROM schema_migrations ORDER BY version ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var statuses []MigrationStatus
	for rows.Next() {
		var s MigrationStatus
		if err := rows.Scan(&s.Version, &s.Dirty, &s.AppliedAt); err != nil {
			return nil, err
		}
		s.Applied = true
		statuses = append(statuses, s)
	}
	return statuses, rows.Err()
}

// markDirty marks a migration version as in-progress.
func (r *Runner) markDirty(ctx context.Context, version int64) error {
	_, err := r.pool.Exec(ctx,
		"INSERT INTO schema_migrations (version, dirty) VALUES ($1, true) ON CONFLICT (version) DO UPDATE SET dirty = true",
		version)
	return err
}

// markClean marks a migration version as successfully applied.
func (r *Runner) markClean(ctx context.Context, version int64) error {
	_, err := r.pool.Exec(ctx,
		"UPDATE schema_migrations SET dirty = false, applied_at = NOW() WHERE version = $1",
		version)
	return err
}

// removeMigrationRecord removes the record from schema_migrations.
func (r *Runner) removeMigrationRecord(ctx context.Context, version int64) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", version)
	return err
}

// migrationName returns a formatted migration identifier.
func migrationName(m Migration) string {
	return fmt.Sprintf("%d_%s", m.Version, m.Name)
}

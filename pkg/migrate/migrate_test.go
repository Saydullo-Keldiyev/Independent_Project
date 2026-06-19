package migrate

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// mockMigration creates a test migration that records execution.
func mockMigration(version int64, name string, upErr, downErr error, executed *[]string) Migration {
	return Migration{
		Version: version,
		Name:    name,
		Up: func(ctx context.Context, pool *pgxpool.Pool) error {
			if executed != nil {
				*executed = append(*executed, fmt.Sprintf("up_%d", version))
			}
			return upErr
		},
		Down: func(ctx context.Context, pool *pgxpool.Pool) error {
			if executed != nil {
				*executed = append(*executed, fmt.Sprintf("down_%d", version))
			}
			return downErr
		},
	}
}

func TestNewRunner_SortsMigrations(t *testing.T) {
	migrations := []Migration{
		{Version: 3, Name: "third"},
		{Version: 1, Name: "first"},
		{Version: 2, Name: "second"},
	}

	runner := NewRunner(nil, migrations, Config{Logger: zap.NewNop()})

	if len(runner.migrations) != 3 {
		t.Fatalf("expected 3 migrations, got %d", len(runner.migrations))
	}
	if runner.migrations[0].Version != 1 {
		t.Errorf("expected first migration version 1, got %d", runner.migrations[0].Version)
	}
	if runner.migrations[1].Version != 2 {
		t.Errorf("expected second migration version 2, got %d", runner.migrations[1].Version)
	}
	if runner.migrations[2].Version != 3 {
		t.Errorf("expected third migration version 3, got %d", runner.migrations[2].Version)
	}
}

func TestNewRunner_DefaultConfig(t *testing.T) {
	runner := NewRunner(nil, nil, Config{})

	if runner.config.Timeout != 300*time.Second {
		t.Errorf("expected timeout 300s, got %s", runner.config.Timeout)
	}
	if runner.config.RollbackTimeout != 120*time.Second {
		t.Errorf("expected rollback timeout 120s, got %s", runner.config.RollbackTimeout)
	}
	if runner.config.Logger == nil {
		t.Error("expected non-nil logger")
	}
}

func TestDefaultConfig(t *testing.T) {
	logger := zap.NewNop()
	cfg := DefaultConfig(logger)

	if cfg.Timeout != 300*time.Second {
		t.Errorf("expected timeout 300s, got %s", cfg.Timeout)
	}
	if cfg.RollbackTimeout != 120*time.Second {
		t.Errorf("expected rollback timeout 120s, got %s", cfg.RollbackTimeout)
	}
	if cfg.Logger != logger {
		t.Error("expected logger to be the one provided")
	}
}

func TestMigrationName(t *testing.T) {
	m := Migration{Version: 20240101120000, Name: "create_users"}
	name := migrationName(m)
	expected := "20240101120000_create_users"
	if name != expected {
		t.Errorf("expected %q, got %q", expected, name)
	}
}

func TestResult_Fields(t *testing.T) {
	result := &Result{
		Applied:         []int64{1, 2, 3},
		RolledBack:      []int64{3},
		Error:           fmt.Errorf("test error"),
		FailedMigration: "3_create_orders",
		Duration:        5 * time.Second,
	}

	if len(result.Applied) != 3 {
		t.Errorf("expected 3 applied, got %d", len(result.Applied))
	}
	if len(result.RolledBack) != 1 {
		t.Errorf("expected 1 rolled back, got %d", len(result.RolledBack))
	}
	if result.Error == nil {
		t.Error("expected non-nil error")
	}
	if result.FailedMigration != "3_create_orders" {
		t.Errorf("expected '3_create_orders', got %q", result.FailedMigration)
	}
}

func TestMigrationStatus_Struct(t *testing.T) {
	now := time.Now()
	s := MigrationStatus{
		Version:   1,
		Name:      "test",
		Applied:   true,
		Dirty:     false,
		AppliedAt: now,
	}

	if s.Version != 1 {
		t.Errorf("expected version 1, got %d", s.Version)
	}
	if !s.Applied {
		t.Error("expected applied to be true")
	}
	if s.Dirty {
		t.Error("expected dirty to be false")
	}
}

func TestDirection_Constants(t *testing.T) {
	if DirectionUp != "up" {
		t.Errorf("expected 'up', got %q", DirectionUp)
	}
	if DirectionDown != "down" {
		t.Errorf("expected 'down', got %q", DirectionDown)
	}
}

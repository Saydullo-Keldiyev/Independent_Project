// Package saga implements the Saga orchestrator pattern for multi-step distributed
// transactions. It supports generic sagas with ordered execution and reverse-order
// compensation on failure, persisting state in PostgreSQL for durability.
package saga

import (
	"context"
	"time"
)

// State represents the current state of a saga execution.
type State string

const (
	StateRunning      State = "running"
	StateCompleted    State = "completed"
	StateCompensating State = "compensating"
	StateFailed       State = "failed"
)

// StepStatus represents the execution status of a saga step.
type StepStatus string

const (
	StepPending     StepStatus = "pending"
	StepCompleted   StepStatus = "completed"
	StepFailed      StepStatus = "failed"
	StepCompensated StepStatus = "compensated"
)

// Step defines a single step in a saga with its execute and compensate functions.
type Step struct {
	Name       string
	Execute    func(ctx context.Context, data map[string]interface{}) error
	Compensate func(ctx context.Context, data map[string]interface{}) error
	Timeout    time.Duration // Per-step timeout; defaults to 30s if zero.
}

// Definition holds the saga type name and its ordered steps.
type Definition struct {
	Type  string
	Steps []Step
}

// Instance represents a running saga instance with its state and data.
type Instance struct {
	ID          string
	Type        string
	ReferenceID string
	State       State
	CurrentStep int
	Data        map[string]interface{}
	StartedAt   time.Time
	CompletedAt *time.Time
	Error       string
}

// StepRecord stores the execution record of a saga step in persistence.
type StepRecord struct {
	ID         string
	SagaID     string
	StepIndex  int
	StepName   string
	Status     StepStatus
	ExecutedAt *time.Time
	Error      string
}

// Orchestrator executes sagas with persistence and compensation support.
type Orchestrator interface {
	// Execute runs all steps of a saga in order. On failure, it compensates.
	Execute(ctx context.Context, definition *Definition, referenceID string, data map[string]interface{}) error
}

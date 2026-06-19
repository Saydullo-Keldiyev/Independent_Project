package saga

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
)

const (
	// DefaultStepTimeout is the maximum time a single step can take before
	// being marked as failed and triggering compensation. (Requirement 18.7)
	DefaultStepTimeout = 30 * time.Second

	// CompensationDeadline is the maximum total time allowed for all
	// compensating transactions to complete. (Requirement 18.4)
	CompensationDeadline = 60 * time.Second
)

// DefaultOrchestrator implements the Orchestrator interface with persistent
// state tracking and ordered compensation on failure.
type DefaultOrchestrator struct {
	store *Store
	log   *zap.Logger
}

// NewOrchestrator creates an orchestrator with persistence and logging.
func NewOrchestrator(store *Store, log *zap.Logger) *DefaultOrchestrator {
	return &DefaultOrchestrator{
		store: store,
		log:   log,
	}
}

// Execute runs all steps of a saga definition in order. If any step fails or
// times out, it compensates previously completed steps in reverse order.
//
// Requirements satisfied:
//   - 18.3: Settlement saga order (charge → credit → release → publish)
//   - 18.4: Compensating transactions in reverse order within 60 seconds
//   - 18.7: Step timeout of 30 seconds triggers failure and compensation
func (o *DefaultOrchestrator) Execute(ctx context.Context, definition *Definition, referenceID string, data map[string]interface{}) error {
	// Create saga instance
	instance := &Instance{
		ID:          NewID(),
		Type:        definition.Type,
		ReferenceID: referenceID,
		State:       StateRunning,
		CurrentStep: 0,
		Data:        data,
		StartedAt:   time.Now().UTC(),
	}

	if err := o.store.CreateSaga(ctx, instance); err != nil {
		return fmt.Errorf("saga create failed: %w", err)
	}

	o.log.Info("saga started",
		zap.String("saga_id", instance.ID),
		zap.String("saga_type", definition.Type),
		zap.String("reference_id", referenceID),
		zap.Int("step_count", len(definition.Steps)),
	)

	// Create step records (all pending initially)
	stepRecords := make([]StepRecord, len(definition.Steps))
	for i, step := range definition.Steps {
		record := StepRecord{
			ID:        NewID(),
			SagaID:    instance.ID,
			StepIndex: i,
			StepName:  step.Name,
			Status:    StepPending,
		}
		if err := o.store.CreateStepRecord(ctx, &record); err != nil {
			return fmt.Errorf("create step record %d: %w", i, err)
		}
		stepRecords[i] = record
	}

	// Execute steps in order
	var failedStep int = -1
	var stepErr error

	for i, step := range definition.Steps {
		instance.CurrentStep = i
		_ = o.store.UpdateSagaState(ctx, instance.ID, StateRunning, i, "", nil)

		timeout := step.Timeout
		if timeout == 0 {
			timeout = DefaultStepTimeout
		}

		o.log.Info("executing saga step",
			zap.String("saga_id", instance.ID),
			zap.Int("step_index", i),
			zap.String("step_name", step.Name),
			zap.Duration("timeout", timeout),
		)

		err := o.executeStepWithTimeout(ctx, step, data, timeout)
		now := time.Now().UTC()

		if err != nil {
			// Step failed or timed out
			failedStep = i
			stepErr = err

			_ = o.store.UpdateStepStatus(ctx, stepRecords[i].ID, StepFailed, &now, err.Error())

			o.log.Error("saga step failed",
				zap.String("saga_id", instance.ID),
				zap.Int("step_index", i),
				zap.String("step_name", step.Name),
				zap.Error(err),
			)
			break
		}

		// Step completed successfully
		_ = o.store.UpdateStepStatus(ctx, stepRecords[i].ID, StepCompleted, &now, "")

		o.log.Info("saga step completed",
			zap.String("saga_id", instance.ID),
			zap.Int("step_index", i),
			zap.String("step_name", step.Name),
		)
	}

	if failedStep == -1 {
		// All steps completed successfully
		now := time.Now().UTC()
		_ = o.store.UpdateSagaState(ctx, instance.ID, StateCompleted, len(definition.Steps)-1, "", &now)

		o.log.Info("saga completed successfully",
			zap.String("saga_id", instance.ID),
			zap.String("saga_type", definition.Type),
		)
		return nil
	}

	// Compensation required: compensate steps in reverse order
	compErr := o.compensate(ctx, definition, instance, stepRecords, failedStep)
	if compErr != nil {
		o.log.Error("saga compensation failed",
			zap.String("saga_id", instance.ID),
			zap.Error(compErr),
		)
		now := time.Now().UTC()
		errMsg := fmt.Sprintf("step %d failed: %v; compensation failed: %v", failedStep, stepErr, compErr)
		_ = o.store.UpdateSagaState(ctx, instance.ID, StateFailed, failedStep, errMsg, &now)
		return fmt.Errorf("saga failed at step %d (%s) and compensation failed: %w", failedStep, definition.Steps[failedStep].Name, compErr)
	}

	now := time.Now().UTC()
	errMsg := fmt.Sprintf("step %d (%s) failed: %v", failedStep, definition.Steps[failedStep].Name, stepErr)
	_ = o.store.UpdateSagaState(ctx, instance.ID, StateFailed, failedStep, errMsg, &now)

	return fmt.Errorf("saga failed at step %d (%s): %w", failedStep, definition.Steps[failedStep].Name, stepErr)
}

// executeStepWithTimeout runs a step function with the configured timeout.
// If the step doesn't complete within the timeout, it returns an error.
func (o *DefaultOrchestrator) executeStepWithTimeout(ctx context.Context, step Step, data map[string]interface{}, timeout time.Duration) error {
	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- step.Execute(stepCtx, data)
	}()

	select {
	case err := <-done:
		return err
	case <-stepCtx.Done():
		if stepCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("step %q timed out after %v", step.Name, timeout)
		}
		return stepCtx.Err()
	}
}

// compensate runs compensation functions for completed steps in reverse order.
// It must complete all compensations within CompensationDeadline (60 seconds).
func (o *DefaultOrchestrator) compensate(ctx context.Context, definition *Definition, instance *Instance, stepRecords []StepRecord, failedStep int) error {
	_ = o.store.UpdateSagaState(ctx, instance.ID, StateCompensating, failedStep, "", nil)

	o.log.Info("starting saga compensation",
		zap.String("saga_id", instance.ID),
		zap.Int("failed_step", failedStep),
		zap.Int("steps_to_compensate", failedStep),
	)

	compCtx, cancel := context.WithTimeout(ctx, CompensationDeadline)
	defer cancel()

	// Compensate completed steps in reverse order (from failedStep-1 down to 0)
	var lastErr error
	for i := failedStep - 1; i >= 0; i-- {
		step := definition.Steps[i]
		if step.Compensate == nil {
			o.log.Warn("no compensation function for step, skipping",
				zap.String("saga_id", instance.ID),
				zap.Int("step_index", i),
				zap.String("step_name", step.Name),
			)
			continue
		}

		o.log.Info("compensating saga step",
			zap.String("saga_id", instance.ID),
			zap.Int("step_index", i),
			zap.String("step_name", step.Name),
		)

		err := step.Compensate(compCtx, instance.Data)
		now := time.Now().UTC()

		if err != nil {
			lastErr = err
			_ = o.store.UpdateStepStatus(ctx, stepRecords[i].ID, StepFailed, &now, fmt.Sprintf("compensation failed: %v", err))
			o.log.Error("step compensation failed",
				zap.String("saga_id", instance.ID),
				zap.Int("step_index", i),
				zap.String("step_name", step.Name),
				zap.Error(err),
			)
			// Continue compensating other steps even if one fails
			continue
		}

		_ = o.store.UpdateStepStatus(ctx, stepRecords[i].ID, StepCompensated, &now, "")
		o.log.Info("step compensated",
			zap.String("saga_id", instance.ID),
			zap.Int("step_index", i),
			zap.String("step_name", step.Name),
		)
	}

	return lastErr
}

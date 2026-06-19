package saga

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store handles persistence of saga state and step records in PostgreSQL.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a new Store with the given connection pool.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// CreateSaga inserts a new saga instance into the sagas table.
func (s *Store) CreateSaga(ctx context.Context, instance *Instance) error {
	query := `
		INSERT INTO sagas (id, saga_type, reference_id, state, current_step, data, started_at, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := s.db.Exec(ctx, query,
		instance.ID,
		instance.Type,
		instance.ReferenceID,
		string(instance.State),
		instance.CurrentStep,
		instance.Data,
		instance.StartedAt,
		instance.Error,
	)
	if err != nil {
		return fmt.Errorf("create saga: %w", err)
	}
	return nil
}

// UpdateSagaState updates the state, current_step, error, and completed_at of a saga.
func (s *Store) UpdateSagaState(ctx context.Context, sagaID string, state State, currentStep int, errMsg string, completedAt *time.Time) error {
	query := `
		UPDATE sagas
		SET state = $1, current_step = $2, error_message = $3, completed_at = $4
		WHERE id = $5
	`
	_, err := s.db.Exec(ctx, query, string(state), currentStep, errMsg, completedAt, sagaID)
	if err != nil {
		return fmt.Errorf("update saga state: %w", err)
	}
	return nil
}

// CreateStepRecord inserts a new step record into the saga_steps table.
func (s *Store) CreateStepRecord(ctx context.Context, record *StepRecord) error {
	query := `
		INSERT INTO saga_steps (id, saga_id, step_index, step_name, status, executed_at, error)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := s.db.Exec(ctx, query,
		record.ID,
		record.SagaID,
		record.StepIndex,
		record.StepName,
		string(record.Status),
		record.ExecutedAt,
		record.Error,
	)
	if err != nil {
		return fmt.Errorf("create step record: %w", err)
	}
	return nil
}

// UpdateStepStatus updates the status, executed_at, and error of a saga step.
func (s *Store) UpdateStepStatus(ctx context.Context, stepID string, status StepStatus, executedAt *time.Time, errMsg string) error {
	query := `
		UPDATE saga_steps
		SET status = $1, executed_at = $2, error = $3
		WHERE id = $4
	`
	_, err := s.db.Exec(ctx, query, string(status), executedAt, errMsg, stepID)
	if err != nil {
		return fmt.Errorf("update step status: %w", err)
	}
	return nil
}

// GetSaga retrieves a saga instance by ID.
func (s *Store) GetSaga(ctx context.Context, sagaID string) (*Instance, error) {
	query := `
		SELECT id, saga_type, reference_id, state, current_step, data, started_at, completed_at, error_message
		FROM sagas WHERE id = $1
	`
	var inst Instance
	err := s.db.QueryRow(ctx, query, sagaID).Scan(
		&inst.ID,
		&inst.Type,
		&inst.ReferenceID,
		&inst.State,
		&inst.CurrentStep,
		&inst.Data,
		&inst.StartedAt,
		&inst.CompletedAt,
		&inst.Error,
	)
	if err != nil {
		return nil, fmt.Errorf("get saga: %w", err)
	}
	return &inst, nil
}

// GetStepRecords returns all step records for a saga ordered by step_index.
func (s *Store) GetStepRecords(ctx context.Context, sagaID string) ([]StepRecord, error) {
	query := `
		SELECT id, saga_id, step_index, step_name, status, executed_at, error
		FROM saga_steps WHERE saga_id = $1 ORDER BY step_index ASC
	`
	rows, err := s.db.Query(ctx, query, sagaID)
	if err != nil {
		return nil, fmt.Errorf("get step records: %w", err)
	}
	defer rows.Close()

	var records []StepRecord
	for rows.Next() {
		var r StepRecord
		if err := rows.Scan(&r.ID, &r.SagaID, &r.StepIndex, &r.StepName, &r.Status, &r.ExecutedAt, &r.Error); err != nil {
			return nil, fmt.Errorf("scan step record: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// NewID generates a new UUID for saga/step IDs.
func NewID() string {
	return uuid.New().String()
}

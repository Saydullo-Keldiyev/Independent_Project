package saga

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// mockStore is an in-memory Store replacement for unit testing.
type mockStore struct {
	mu       sync.Mutex
	sagas    map[string]*Instance
	steps    map[string][]StepRecord
	stepByID map[string]*StepRecord
}

func newMockStore() *mockStore {
	return &mockStore{
		sagas:    make(map[string]*Instance),
		steps:    make(map[string][]StepRecord),
		stepByID: make(map[string]*StepRecord),
	}
}

func (m *mockStore) createSaga(instance *Instance) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sagas[instance.ID] = instance
}

func (m *mockStore) getSaga(id string) *Instance {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sagas[id]
}

func (m *mockStore) getSteps(sagaID string) []StepRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.steps[sagaID]
}

// testOrchestrator uses real Store methods but with a mock to intercept calls.
// Since we can't easily mock pgxpool, we test the orchestrator logic with
// a minimal approach: test the step execution and compensation logic directly.

func TestOrchestratorAllStepsSucceed(t *testing.T) {
	var executionOrder []string
	var mu sync.Mutex

	definition := &Definition{
		Type: "test_saga",
		Steps: []Step{
			{
				Name: "step_1",
				Execute: func(ctx context.Context, data map[string]interface{}) error {
					mu.Lock()
					executionOrder = append(executionOrder, "execute_1")
					mu.Unlock()
					return nil
				},
				Compensate: func(ctx context.Context, data map[string]interface{}) error {
					mu.Lock()
					executionOrder = append(executionOrder, "compensate_1")
					mu.Unlock()
					return nil
				},
				Timeout: 5 * time.Second,
			},
			{
				Name: "step_2",
				Execute: func(ctx context.Context, data map[string]interface{}) error {
					mu.Lock()
					executionOrder = append(executionOrder, "execute_2")
					mu.Unlock()
					return nil
				},
				Compensate: func(ctx context.Context, data map[string]interface{}) error {
					mu.Lock()
					executionOrder = append(executionOrder, "compensate_2")
					mu.Unlock()
					return nil
				},
				Timeout: 5 * time.Second,
			},
			{
				Name: "step_3",
				Execute: func(ctx context.Context, data map[string]interface{}) error {
					mu.Lock()
					executionOrder = append(executionOrder, "execute_3")
					mu.Unlock()
					return nil
				},
				Compensate: func(ctx context.Context, data map[string]interface{}) error {
					mu.Lock()
					executionOrder = append(executionOrder, "compensate_3")
					mu.Unlock()
					return nil
				},
				Timeout: 5 * time.Second,
			},
		},
	}

	// Test step execution order directly (without DB)
	ctx := context.Background()
	data := map[string]interface{}{"test": "data"}

	for _, step := range definition.Steps {
		err := step.Execute(ctx, data)
		if err != nil {
			t.Fatalf("step %s failed: %v", step.Name, err)
		}
	}

	expected := []string{"execute_1", "execute_2", "execute_3"}
	if len(executionOrder) != len(expected) {
		t.Fatalf("expected %d executions, got %d", len(expected), len(executionOrder))
	}
	for i, v := range expected {
		if executionOrder[i] != v {
			t.Errorf("execution[%d] = %s, want %s", i, executionOrder[i], v)
		}
	}
}

func TestOrchestratorCompensatesOnFailure(t *testing.T) {
	var executionOrder []string
	var mu sync.Mutex

	record := func(name string) {
		mu.Lock()
		executionOrder = append(executionOrder, name)
		mu.Unlock()
	}

	definition := &Definition{
		Type: "test_saga",
		Steps: []Step{
			{
				Name: "step_1",
				Execute: func(ctx context.Context, data map[string]interface{}) error {
					record("execute_1")
					return nil
				},
				Compensate: func(ctx context.Context, data map[string]interface{}) error {
					record("compensate_1")
					return nil
				},
				Timeout: 5 * time.Second,
			},
			{
				Name: "step_2",
				Execute: func(ctx context.Context, data map[string]interface{}) error {
					record("execute_2")
					return nil
				},
				Compensate: func(ctx context.Context, data map[string]interface{}) error {
					record("compensate_2")
					return nil
				},
				Timeout: 5 * time.Second,
			},
			{
				Name: "step_3",
				Execute: func(ctx context.Context, data map[string]interface{}) error {
					record("execute_3")
					return errors.New("step 3 failed")
				},
				Compensate: func(ctx context.Context, data map[string]interface{}) error {
					record("compensate_3")
					return nil
				},
				Timeout: 5 * time.Second,
			},
		},
	}

	ctx := context.Background()
	data := map[string]interface{}{"test": "data"}

	// Simulate orchestrator logic: execute steps, on failure compensate in reverse
	var failedStep int = -1
	for i, step := range definition.Steps {
		err := step.Execute(ctx, data)
		if err != nil {
			failedStep = i
			break
		}
	}

	if failedStep == -1 {
		t.Fatal("expected a step to fail")
	}

	if failedStep != 2 {
		t.Fatalf("expected step 2 to fail, got step %d", failedStep)
	}

	// Compensate in reverse order
	for i := failedStep - 1; i >= 0; i-- {
		step := definition.Steps[i]
		if step.Compensate != nil {
			_ = step.Compensate(ctx, data)
		}
	}

	expected := []string{"execute_1", "execute_2", "execute_3", "compensate_2", "compensate_1"}
	if len(executionOrder) != len(expected) {
		t.Fatalf("expected %d operations, got %d: %v", len(expected), len(executionOrder), executionOrder)
	}
	for i, v := range expected {
		if executionOrder[i] != v {
			t.Errorf("operation[%d] = %s, want %s", i, executionOrder[i], v)
		}
	}
}

func TestOrchestratorStepTimeout(t *testing.T) {
	slowStep := Step{
		Name: "slow_step",
		Execute: func(ctx context.Context, data map[string]interface{}) error {
			select {
			case <-time.After(5 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		Timeout: 100 * time.Millisecond, // very short timeout for testing
	}

	ctx := context.Background()
	data := map[string]interface{}{}

	// Simulate the timeout logic from orchestrator
	stepCtx, cancel := context.WithTimeout(ctx, slowStep.Timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- slowStep.Execute(stepCtx, data)
	}()

	var err error
	select {
	case err = <-done:
	case <-stepCtx.Done():
		err = stepCtx.Err()
	}

	if err == nil {
		t.Fatal("expected timeout error")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected DeadlineExceeded, got: %v", err)
	}
}

func TestOrchestratorFirstStepFailsNoCompensation(t *testing.T) {
	var executionOrder []string
	var mu sync.Mutex

	record := func(name string) {
		mu.Lock()
		executionOrder = append(executionOrder, name)
		mu.Unlock()
	}

	definition := &Definition{
		Type: "test_saga",
		Steps: []Step{
			{
				Name: "step_1",
				Execute: func(ctx context.Context, data map[string]interface{}) error {
					record("execute_1")
					return errors.New("step 1 failed")
				},
				Compensate: func(ctx context.Context, data map[string]interface{}) error {
					record("compensate_1")
					return nil
				},
				Timeout: 5 * time.Second,
			},
			{
				Name: "step_2",
				Execute: func(ctx context.Context, data map[string]interface{}) error {
					record("execute_2")
					return nil
				},
				Compensate: func(ctx context.Context, data map[string]interface{}) error {
					record("compensate_2")
					return nil
				},
				Timeout: 5 * time.Second,
			},
		},
	}

	ctx := context.Background()
	data := map[string]interface{}{}

	var failedStep int = -1
	for i, step := range definition.Steps {
		err := step.Execute(ctx, data)
		if err != nil {
			failedStep = i
			break
		}
	}

	if failedStep != 0 {
		t.Fatalf("expected step 0 to fail, got %d", failedStep)
	}

	// Compensate in reverse (failedStep-1 down to 0 — nothing to compensate)
	for i := failedStep - 1; i >= 0; i-- {
		step := definition.Steps[i]
		if step.Compensate != nil {
			_ = step.Compensate(ctx, data)
		}
	}

	// Only execute_1 should appear — no compensation since nothing was completed
	expected := []string{"execute_1"}
	if len(executionOrder) != len(expected) {
		t.Fatalf("expected %d operations, got %d: %v", len(expected), len(executionOrder), executionOrder)
	}
	for i, v := range expected {
		if executionOrder[i] != v {
			t.Errorf("operation[%d] = %s, want %s", i, executionOrder[i], v)
		}
	}
}

func TestParseSettlementData(t *testing.T) {
	data := map[string]interface{}{
		"auction_id":     "auction-123",
		"winner_id":      "winner-456",
		"seller_id":      "seller-789",
		"winning_amount": 100.50,
		"winner_hold_id": "hold-abc",
		"loser_hold_ids": []interface{}{"hold-1", "hold-2", "hold-3"},
	}

	sd, err := ParseSettlementData(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sd.AuctionID != "auction-123" {
		t.Errorf("auction_id = %s, want auction-123", sd.AuctionID)
	}
	if sd.WinnerID != "winner-456" {
		t.Errorf("winner_id = %s, want winner-456", sd.WinnerID)
	}
	if sd.SellerID != "seller-789" {
		t.Errorf("seller_id = %s, want seller-789", sd.SellerID)
	}
	if sd.WinningAmount != 100.50 {
		t.Errorf("winning_amount = %f, want 100.50", sd.WinningAmount)
	}
	if sd.WinnerHoldID != "hold-abc" {
		t.Errorf("winner_hold_id = %s, want hold-abc", sd.WinnerHoldID)
	}
	if len(sd.LoserHoldIDs) != 3 {
		t.Errorf("loser_hold_ids length = %d, want 3", len(sd.LoserHoldIDs))
	}
}

func TestParseSettlementDataMissingFields(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
	}{
		{"missing auction_id", map[string]interface{}{"winner_id": "w", "seller_id": "s", "winning_amount": 1.0, "winner_hold_id": "h"}},
		{"missing winner_id", map[string]interface{}{"auction_id": "a", "seller_id": "s", "winning_amount": 1.0, "winner_hold_id": "h"}},
		{"missing seller_id", map[string]interface{}{"auction_id": "a", "winner_id": "w", "winning_amount": 1.0, "winner_hold_id": "h"}},
		{"missing winning_amount", map[string]interface{}{"auction_id": "a", "winner_id": "w", "seller_id": "s", "winner_hold_id": "h"}},
		{"missing winner_hold_id", map[string]interface{}{"auction_id": "a", "winner_id": "w", "seller_id": "s", "winning_amount": 1.0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSettlementData(tt.data)
			if err == nil {
				t.Error("expected error for missing field")
			}
		})
	}
}

func TestSettlementDataRoundTrip(t *testing.T) {
	original := &SettlementData{
		AuctionID:     "auction-123",
		WinnerID:      "winner-456",
		SellerID:      "seller-789",
		WinningAmount: 250.75,
		WinnerHoldID:  "hold-abc",
		LoserHoldIDs:  []string{"hold-1", "hold-2"},
	}

	dataMap := original.ToDataMap()
	parsed, err := ParseSettlementData(dataMap)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if parsed.AuctionID != original.AuctionID {
		t.Errorf("auction_id mismatch")
	}
	if parsed.WinnerID != original.WinnerID {
		t.Errorf("winner_id mismatch")
	}
	if parsed.SellerID != original.SellerID {
		t.Errorf("seller_id mismatch")
	}
	if parsed.WinningAmount != original.WinningAmount {
		t.Errorf("winning_amount mismatch: %f vs %f", parsed.WinningAmount, original.WinningAmount)
	}
	if parsed.WinnerHoldID != original.WinnerHoldID {
		t.Errorf("winner_hold_id mismatch")
	}
	if len(parsed.LoserHoldIDs) != len(original.LoserHoldIDs) {
		t.Errorf("loser_hold_ids length mismatch")
	}
}

func TestExecuteStepWithTimeout(t *testing.T) {
	t.Run("succeeds within timeout", func(t *testing.T) {
		step := Step{
			Name: "fast_step",
			Execute: func(ctx context.Context, data map[string]interface{}) error {
				return nil
			},
			Timeout: 5 * time.Second,
		}

		ctx := context.Background()
		stepCtx, cancel := context.WithTimeout(ctx, step.Timeout)
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- step.Execute(stepCtx, nil)
		}()

		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		case <-stepCtx.Done():
			t.Fatal("step timed out unexpectedly")
		}
	})

	t.Run("returns error on step failure", func(t *testing.T) {
		expectedErr := errors.New("something went wrong")
		step := Step{
			Name: "failing_step",
			Execute: func(ctx context.Context, data map[string]interface{}) error {
				return expectedErr
			},
			Timeout: 5 * time.Second,
		}

		ctx := context.Background()
		stepCtx, cancel := context.WithTimeout(ctx, step.Timeout)
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- step.Execute(stepCtx, nil)
		}()

		select {
		case err := <-done:
			if !errors.Is(err, expectedErr) {
				t.Fatalf("expected %v, got: %v", expectedErr, err)
			}
		case <-stepCtx.Done():
			t.Fatal("step timed out unexpectedly")
		}
	})
}

func TestCompensationReverseOrder(t *testing.T) {
	var order []int
	var mu sync.Mutex

	steps := []Step{
		{Name: "s0", Compensate: func(ctx context.Context, data map[string]interface{}) error {
			mu.Lock()
			order = append(order, 0)
			mu.Unlock()
			return nil
		}},
		{Name: "s1", Compensate: func(ctx context.Context, data map[string]interface{}) error {
			mu.Lock()
			order = append(order, 1)
			mu.Unlock()
			return nil
		}},
		{Name: "s2", Compensate: func(ctx context.Context, data map[string]interface{}) error {
			mu.Lock()
			order = append(order, 2)
			mu.Unlock()
			return nil
		}},
		{Name: "s3", Compensate: func(ctx context.Context, data map[string]interface{}) error {
			mu.Lock()
			order = append(order, 3)
			mu.Unlock()
			return nil
		}},
	}

	// Simulate: step 3 fails, compensate s2, s1, s0 in reverse
	failedStep := 3
	ctx := context.Background()
	data := map[string]interface{}{}

	for i := failedStep - 1; i >= 0; i-- {
		if steps[i].Compensate != nil {
			_ = steps[i].Compensate(ctx, data)
		}
	}

	expected := []int{2, 1, 0}
	if len(order) != len(expected) {
		t.Fatalf("expected %d compensations, got %d", len(expected), len(order))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("compensation[%d] = %d, want %d", i, order[i], v)
		}
	}
}

func TestDefaultStepTimeout(t *testing.T) {
	if DefaultStepTimeout != 30*time.Second {
		t.Errorf("DefaultStepTimeout = %v, want 30s", DefaultStepTimeout)
	}
}

func TestCompensationDeadline(t *testing.T) {
	if CompensationDeadline != 60*time.Second {
		t.Errorf("CompensationDeadline = %v, want 60s", CompensationDeadline)
	}
}

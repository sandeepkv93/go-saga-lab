package runtime

import (
	"testing"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
)

func TestParallelExecutionAllStepsSucceed(t *testing.T) {
	t.Parallel()

	exec, err := NewParallelExecution([]string{"reserve_inventory", "authorize_payment"})
	if err != nil {
		t.Fatalf("NewParallelExecution() error = %v", err)
	}
	exec.StartAll()

	status, err := exec.RecordStepResult("reserve_inventory", true)
	if err != nil {
		t.Fatalf("RecordStepResult() error = %v", err)
	}
	if status != domain.SagaStatusRunning {
		t.Fatalf("status after first success = %q, want %q", status, domain.SagaStatusRunning)
	}

	status, err = exec.RecordStepResult("authorize_payment", true)
	if err != nil {
		t.Fatalf("RecordStepResult() error = %v", err)
	}
	if status != domain.SagaStatusCompleted {
		t.Fatalf("status after final success = %q, want %q", status, domain.SagaStatusCompleted)
	}
}

func TestParallelExecutionFailureTriggersCompensation(t *testing.T) {
	t.Parallel()

	exec, err := NewParallelExecution([]string{"reserve_inventory", "authorize_payment"})
	if err != nil {
		t.Fatalf("NewParallelExecution() error = %v", err)
	}
	exec.StartAll()

	status, err := exec.RecordStepResult("authorize_payment", false)
	if err != nil {
		t.Fatalf("RecordStepResult() error = %v", err)
	}
	if status != domain.SagaStatusCompensating {
		t.Fatalf("status after failure = %q, want %q", status, domain.SagaStatusCompensating)
	}
}

func TestParallelExecutionRejectsInvalidOperations(t *testing.T) {
	t.Parallel()

	if _, err := NewParallelExecution(nil); err == nil {
		t.Fatal("expected error for empty steps")
	}

	exec, err := NewParallelExecution([]string{"reserve_inventory"})
	if err != nil {
		t.Fatalf("NewParallelExecution() error = %v", err)
	}

	exec.StartAll()
	if _, err := exec.RecordStepResult("missing", true); err == nil {
		t.Fatal("expected error for unknown step")
	}

	if _, err := exec.RecordStepResult("reserve_inventory", true); err != nil {
		t.Fatalf("RecordStepResult() error = %v", err)
	}
	if _, err := exec.RecordStepResult("reserve_inventory", true); err == nil {
		t.Fatal("expected error for duplicate completion")
	}
}

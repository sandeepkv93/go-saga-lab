package memory

import (
	"context"
	"testing"
	"time"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
)

func TestCreateListAndUpdateStepExecutions(t *testing.T) {
	t.Parallel()

	repo := New()
	now := time.Now().UTC()
	instance := domain.SagaInstance{
		ID:             "saga-steps-1",
		TemplateID:     "order-flow",
		Status:         domain.SagaStatusCreated,
		InputJSON:      []byte(`{}`),
		IdempotencyKey: "idem-steps-1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := repo.CreateSagaInstance(context.Background(), instance); err != nil {
		t.Fatalf("CreateSagaInstance() error = %v", err)
	}

	steps := []domain.StepExecution{
		{
			StepName:   "reserve_inventory",
			BranchName: "branch-a",
			Status:     domain.StepStatusPending,
			Attempts:   0,
			StartedAt:  now,
			UpdatedAt:  now,
		},
		{
			StepName:   "authorize_payment",
			BranchName: "branch-b",
			Status:     domain.StepStatusPending,
			Attempts:   0,
			StartedAt:  now,
			UpdatedAt:  now,
		},
	}
	if err := repo.CreateStepExecutions(context.Background(), instance.ID, steps); err != nil {
		t.Fatalf("CreateStepExecutions() error = %v", err)
	}

	got, err := repo.ListStepExecutions(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("ListStepExecutions() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	finishedAt := time.Now().UTC()
	if err := repo.UpdateStepExecution(context.Background(), instance.ID, "reserve_inventory", domain.StepStatusSucceeded, 1, "", &finishedAt); err != nil {
		t.Fatalf("UpdateStepExecution() error = %v", err)
	}

	got, err = repo.ListStepExecutions(context.Background(), instance.ID)
	if err != nil {
		t.Fatalf("ListStepExecutions() error = %v", err)
	}

	var updated domain.StepExecution
	for _, step := range got {
		if step.StepName == "reserve_inventory" {
			updated = step
		}
	}
	if updated.Status != domain.StepStatusSucceeded {
		t.Fatalf("updated.Status = %q, want %q", updated.Status, domain.StepStatusSucceeded)
	}
	if updated.Attempts != 1 {
		t.Fatalf("updated.Attempts = %d, want 1", updated.Attempts)
	}
	if updated.FinishedAt == nil {
		t.Fatal("expected finished timestamp")
	}
}

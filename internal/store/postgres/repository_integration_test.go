package postgres

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
)

func TestRepositoryCreateAndGetSagaInstance(t *testing.T) {
	t.Parallel()

	databaseURL := os.Getenv("POSTGRES_DSN")
	if databaseURL == "" {
		t.Skip("POSTGRES_DSN is not set")
	}

	ctx := context.Background()
	repo, err := New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(repo.Close)

	if err := repo.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	instance := domain.SagaInstance{
		ID:             "test-saga-1",
		TemplateID:     "order-flow-v1",
		Status:         domain.SagaStatusCreated,
		InputJSON:      []byte(`{"order_id":"o-123"}`),
		IdempotencyKey: "idem-1",
		CreatedAt:      time.Now().UTC().Round(time.Microsecond),
		UpdatedAt:      time.Now().UTC().Round(time.Microsecond),
	}

	if err := repo.CreateSagaInstance(ctx, instance); err != nil {
		t.Fatalf("CreateSagaInstance() error = %v", err)
	}

	got, err := repo.GetSagaInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("GetSagaInstance() error = %v", err)
	}

	if got.ID != instance.ID {
		t.Fatalf("GetSagaInstance().ID = %q, want %q", got.ID, instance.ID)
	}
	if got.TemplateID != instance.TemplateID {
		t.Fatalf("GetSagaInstance().TemplateID = %q, want %q", got.TemplateID, instance.TemplateID)
	}
	if got.Status != instance.Status {
		t.Fatalf("GetSagaInstance().Status = %q, want %q", got.Status, instance.Status)
	}

	if err := repo.UpdateSagaStatus(ctx, instance.ID, domain.SagaStatusRunning); err != nil {
		t.Fatalf("UpdateSagaStatus() error = %v", err)
	}

	got, err = repo.GetSagaInstance(ctx, instance.ID)
	if err != nil {
		t.Fatalf("GetSagaInstance() after update error = %v", err)
	}
	if got.Status != domain.SagaStatusRunning {
		t.Fatalf("GetSagaInstance().Status after update = %q, want %q", got.Status, domain.SagaStatusRunning)
	}
}

func TestRepositoryCreateSagaInstanceWithOutbox(t *testing.T) {
	t.Parallel()

	databaseURL := os.Getenv("POSTGRES_DSN")
	if databaseURL == "" {
		t.Skip("POSTGRES_DSN is not set")
	}

	ctx := context.Background()
	repo, err := New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(repo.Close)

	if err := repo.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	instance := domain.SagaInstance{
		ID:             "test-saga-outbox-1",
		TemplateID:     "order-flow-v1",
		Status:         domain.SagaStatusCreated,
		InputJSON:      []byte(`{"order_id":"o-999"}`),
		IdempotencyKey: "idem-outbox-1",
		CreatedAt:      time.Now().UTC().Round(time.Microsecond),
		UpdatedAt:      time.Now().UTC().Round(time.Microsecond),
	}
	event := domain.OutboxEvent{
		AggregateType: "saga",
		AggregateID:   instance.ID,
		EventType:     "saga.created",
		PayloadJSON:   []byte(`{"saga_id":"test-saga-outbox-1"}`),
		DedupeKey:     "dedupe-outbox-1",
		Status:        "pending",
		CreatedAt:     instance.CreatedAt,
		UpdatedAt:     instance.UpdatedAt,
	}

	if err := repo.CreateSagaInstanceWithOutbox(ctx, instance, event); err != nil {
		t.Fatalf("CreateSagaInstanceWithOutbox() error = %v", err)
	}

	now := time.Now().UTC()
	events, err := repo.ClaimDispatchableOutboxEvents(ctx, now, "test-owner", now.Add(time.Second), 10)
	if err != nil {
		t.Fatalf("ClaimDispatchableOutboxEvents() error = %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one pending outbox event")
	}

	if err := repo.UpdateOutboxEventDelivery(ctx, event.DedupeKey, "published", 1, nil, "test-owner"); err != nil {
		t.Fatalf("UpdateOutboxEventDelivery() error = %v", err)
	}

	events, err = repo.ClaimDispatchableOutboxEvents(ctx, time.Now().UTC(), "test-owner", time.Now().UTC().Add(time.Second), 10)
	if err != nil {
		t.Fatalf("ClaimDispatchableOutboxEvents() after update error = %v", err)
	}
	for _, gotEvent := range events {
		if gotEvent.DedupeKey == event.DedupeKey {
			t.Fatalf("expected dedupe key %q to be absent from claimable events", event.DedupeKey)
		}
	}
}

func TestRepositoryCreateListAndUpdateStepExecutions(t *testing.T) {
	t.Parallel()

	databaseURL := os.Getenv("POSTGRES_DSN")
	if databaseURL == "" {
		t.Skip("POSTGRES_DSN is not set")
	}

	ctx := context.Background()
	repo, err := New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(repo.Close)

	if err := repo.Ping(ctx); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}

	now := time.Now().UTC().Round(time.Microsecond)
	instance := domain.SagaInstance{
		ID:             "test-saga-steps-1",
		TemplateID:     "order-flow-v1",
		Status:         domain.SagaStatusCreated,
		InputJSON:      []byte(`{"order_id":"o-steps-1"}`),
		IdempotencyKey: "idem-steps-1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := repo.CreateSagaInstance(ctx, instance); err != nil {
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
	if err := repo.CreateStepExecutions(ctx, instance.ID, steps); err != nil {
		t.Fatalf("CreateStepExecutions() error = %v", err)
	}

	got, err := repo.ListStepExecutions(ctx, instance.ID)
	if err != nil {
		t.Fatalf("ListStepExecutions() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	finishedAt := time.Now().UTC().Round(time.Microsecond)
	if err := repo.UpdateStepExecution(ctx, instance.ID, "reserve_inventory", domain.StepStatusSucceeded, 1, "", &finishedAt); err != nil {
		t.Fatalf("UpdateStepExecution() error = %v", err)
	}

	got, err = repo.ListStepExecutions(ctx, instance.ID)
	if err != nil {
		t.Fatalf("ListStepExecutions() after update error = %v", err)
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

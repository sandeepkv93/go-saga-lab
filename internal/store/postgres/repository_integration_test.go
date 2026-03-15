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

	events, err := repo.ListDispatchableOutboxEvents(ctx, time.Now().UTC())
	if err != nil {
		t.Fatalf("ListDispatchableOutboxEvents() error = %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least one pending outbox event")
	}

	if err := repo.UpdateOutboxEventDelivery(ctx, event.DedupeKey, "published", 1, nil); err != nil {
		t.Fatalf("UpdateOutboxEventDelivery() error = %v", err)
	}

	events, err = repo.ListDispatchableOutboxEvents(ctx, time.Now().UTC())
	if err != nil {
		t.Fatalf("ListDispatchableOutboxEvents() after update error = %v", err)
	}
	for _, gotEvent := range events {
		if gotEvent.DedupeKey == event.DedupeKey {
			t.Fatalf("expected dedupe key %q to be absent from dispatchable events", event.DedupeKey)
		}
	}
}

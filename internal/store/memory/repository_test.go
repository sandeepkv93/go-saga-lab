package memory

import (
	"context"
	"testing"
	"time"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
)

func TestCreateSagaInstanceWithOutbox(t *testing.T) {
	t.Parallel()

	repo := New()
	now := time.Now().UTC()

	instance := domain.SagaInstance{
		ID:             "saga-memory-1",
		TemplateID:     "order-flow",
		Status:         domain.SagaStatusCreated,
		InputJSON:      []byte(`{"order_id":"o-memory-1"}`),
		IdempotencyKey: "idem-memory-1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	event := domain.OutboxEvent{
		AggregateType: "saga",
		AggregateID:   instance.ID,
		EventType:     "saga.created",
		PayloadJSON:   []byte(`{"saga_id":"saga-memory-1"}`),
		DedupeKey:     "idem-memory-1:saga.created",
		Status:        "pending",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := repo.CreateSagaInstanceWithOutbox(context.Background(), instance, event); err != nil {
		t.Fatalf("CreateSagaInstanceWithOutbox() error = %v", err)
	}

	events, err := repo.ClaimDispatchableOutboxEvents(context.Background(), now, "test-owner", now.Add(time.Second), 10)
	if err != nil {
		t.Fatalf("ClaimDispatchableOutboxEvents() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].DedupeKey != event.DedupeKey {
		t.Fatalf("events[0].DedupeKey = %q, want %q", events[0].DedupeKey, event.DedupeKey)
	}
}

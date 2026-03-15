package outbox

import (
	"context"
	"testing"
	"time"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
	"github.com/sandeepkv93/go-saga-lab/internal/store/memory"
)

func TestRunnerRunOnceDispatchesPendingEvents(t *testing.T) {
	t.Parallel()

	repo := memory.New()
	now := time.Now().UTC()
	instance := domain.SagaInstance{
		ID:             "saga-runner-1",
		TemplateID:     "order-flow",
		Status:         domain.SagaStatusCreated,
		InputJSON:      []byte(`{}`),
		IdempotencyKey: "idem-runner-1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	event := domain.OutboxEvent{
		AggregateType: "saga",
		AggregateID:   instance.ID,
		EventType:     "saga.created",
		PayloadJSON:   []byte(`{"saga_id":"saga-runner-1"}`),
		DedupeKey:     "idem-runner-1:saga.created",
		Status:        "pending",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := repo.CreateSagaInstanceWithOutbox(context.Background(), instance, event); err != nil {
		t.Fatalf("CreateSagaInstanceWithOutbox() error = %v", err)
	}

	dispatcher, err := NewDispatcher(repo, &fakePublisher{}, 100*time.Millisecond, time.Second, "publisher-a", time.Second)
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}
	runner, err := NewRunner(dispatcher, 10*time.Millisecond, true)
	if err != nil {
		t.Fatalf("NewRunner() error = %v", err)
	}

	if err := runner.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	events, err := repo.ClaimDispatchableOutboxEvents(context.Background(), time.Now().UTC(), "publisher-b", time.Now().UTC().Add(time.Second), 10)
	if err != nil {
		t.Fatalf("ClaimDispatchableOutboxEvents() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("len(events) = %d, want 0", len(events))
	}
}

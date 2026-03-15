package outbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
	"github.com/sandeepkv93/go-saga-lab/internal/store/memory"
)

type fakePublisher struct {
	failDedupeKey string
	published     []string
}

func (p *fakePublisher) Publish(_ context.Context, event domain.OutboxEvent) error {
	if event.DedupeKey == p.failDedupeKey {
		return errors.New("publish failed")
	}
	p.published = append(p.published, event.DedupeKey)
	return nil
}

func TestDispatcherPublishesPendingEvents(t *testing.T) {
	t.Parallel()

	repo := memory.New()
	now := time.Now().UTC()
	instance := domain.SagaInstance{
		ID:             "saga-dispatch-1",
		TemplateID:     "order-flow",
		Status:         domain.SagaStatusCreated,
		InputJSON:      []byte(`{}`),
		IdempotencyKey: "idem-dispatch-1",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	event := domain.OutboxEvent{
		AggregateType: "saga",
		AggregateID:   instance.ID,
		EventType:     "saga.created",
		PayloadJSON:   []byte(`{"saga_id":"saga-dispatch-1"}`),
		DedupeKey:     "idem-dispatch-1:saga.created",
		Status:        "pending",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := repo.CreateSagaInstanceWithOutbox(context.Background(), instance, event); err != nil {
		t.Fatalf("CreateSagaInstanceWithOutbox() error = %v", err)
	}

	publisher := &fakePublisher{}
	dispatcher, err := NewDispatcher(repo, publisher, 100*time.Millisecond, time.Second, "publisher-a", time.Second)
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	dispatched, err := dispatcher.DispatchPending(context.Background())
	if err != nil {
		t.Fatalf("DispatchPending() error = %v", err)
	}
	if dispatched != 1 {
		t.Fatalf("DispatchPending() = %d, want %d", dispatched, 1)
	}

	events, err := repo.ClaimDispatchableOutboxEvents(context.Background(), time.Now().UTC(), "publisher-b", time.Now().UTC().Add(time.Second), 10)
	if err != nil {
		t.Fatalf("ClaimDispatchableOutboxEvents() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("len(events) = %d, want 0", len(events))
	}
}

func TestDispatcherMarksFailedEvents(t *testing.T) {
	t.Parallel()

	repo := memory.New()
	now := time.Now().UTC()
	instance := domain.SagaInstance{
		ID:             "saga-dispatch-2",
		TemplateID:     "order-flow",
		Status:         domain.SagaStatusCreated,
		InputJSON:      []byte(`{}`),
		IdempotencyKey: "idem-dispatch-2",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	event := domain.OutboxEvent{
		AggregateType: "saga",
		AggregateID:   instance.ID,
		EventType:     "saga.created",
		PayloadJSON:   []byte(`{"saga_id":"saga-dispatch-2"}`),
		DedupeKey:     "idem-dispatch-2:saga.created",
		Status:        "pending",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := repo.CreateSagaInstanceWithOutbox(context.Background(), instance, event); err != nil {
		t.Fatalf("CreateSagaInstanceWithOutbox() error = %v", err)
	}

	publisher := &fakePublisher{failDedupeKey: event.DedupeKey}
	dispatcher, err := NewDispatcher(repo, publisher, 100*time.Millisecond, time.Second, "publisher-a", 200*time.Millisecond)
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}

	dispatched, err := dispatcher.DispatchPending(context.Background())
	if err != nil {
		t.Fatalf("DispatchPending() error = %v", err)
	}
	if dispatched != 0 {
		t.Fatalf("DispatchPending() = %d, want %d", dispatched, 0)
	}

	events, err := repo.ClaimDispatchableOutboxEvents(context.Background(), time.Now().UTC(), "publisher-b", time.Now().UTC().Add(time.Second), 10)
	if err != nil {
		t.Fatalf("ClaimDispatchableOutboxEvents() error = %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("len(events) = %d, want 0 because failed events should not be immediately dispatchable", len(events))
	}

	events, err = repo.ClaimDispatchableOutboxEvents(context.Background(), time.Now().UTC().Add(time.Second), "publisher-b", time.Now().UTC().Add(2*time.Second), 10)
	if err != nil {
		t.Fatalf("ClaimDispatchableOutboxEvents() after delay error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) after delay = %d, want 1", len(events))
	}
}

func TestDispatcherRequiresLeaseSettings(t *testing.T) {
	t.Parallel()

	repo := memory.New()
	publisher := &fakePublisher{}

	if _, err := NewDispatcher(repo, publisher, time.Second, time.Second, "", time.Second); err == nil {
		t.Fatal("expected error for empty lease owner")
	}
	if _, err := NewDispatcher(repo, publisher, time.Second, time.Second, "owner", 0); err == nil {
		t.Fatal("expected error for zero lease TTL")
	}
}

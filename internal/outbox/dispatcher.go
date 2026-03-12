package outbox

import (
	"context"
	"fmt"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
	"github.com/sandeepkv93/go-saga-lab/internal/store"
)

type Publisher interface {
	Publish(ctx context.Context, event domain.OutboxEvent) error
}

type Dispatcher struct {
	repository store.SagaOutboxRepository
	publisher  Publisher
}

func NewDispatcher(repository store.SagaOutboxRepository, publisher Publisher) (*Dispatcher, error) {
	if repository == nil {
		return nil, fmt.Errorf("repository is required")
	}
	if publisher == nil {
		return nil, fmt.Errorf("publisher is required")
	}

	return &Dispatcher{
		repository: repository,
		publisher:  publisher,
	}, nil
}

func (d *Dispatcher) DispatchPending(ctx context.Context) (int, error) {
	events, err := d.repository.ListPendingOutboxEvents(ctx)
	if err != nil {
		return 0, fmt.Errorf("list pending outbox events: %w", err)
	}

	dispatched := 0
	for _, event := range events {
		nextAttempts := event.Attempts + 1
		if err := d.publisher.Publish(ctx, event); err != nil {
			if markErr := d.repository.MarkOutboxEventStatus(ctx, event.DedupeKey, "failed", nextAttempts); markErr != nil {
				return dispatched, fmt.Errorf("mark failed outbox event: %w", markErr)
			}
			continue
		}

		if err := d.repository.MarkOutboxEventStatus(ctx, event.DedupeKey, "published", nextAttempts); err != nil {
			return dispatched, fmt.Errorf("mark published outbox event: %w", err)
		}
		dispatched++
	}

	return dispatched, nil
}

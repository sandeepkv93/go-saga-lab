package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
	"github.com/sandeepkv93/go-saga-lab/internal/store"
)

type Publisher interface {
	Publish(ctx context.Context, event domain.OutboxEvent) error
}

type Dispatcher struct {
	repository store.SagaOutboxRepository
	publisher  Publisher
	retryBase  time.Duration
	retryMax   time.Duration
}

func NewDispatcher(repository store.SagaOutboxRepository, publisher Publisher, retryBase, retryMax time.Duration) (*Dispatcher, error) {
	if repository == nil {
		return nil, fmt.Errorf("repository is required")
	}
	if publisher == nil {
		return nil, fmt.Errorf("publisher is required")
	}
	if retryBase <= 0 {
		return nil, fmt.Errorf("retry base delay must be positive")
	}
	if retryMax < retryBase {
		return nil, fmt.Errorf("retry max delay must be greater than or equal to retry base delay")
	}

	return &Dispatcher{
		repository: repository,
		publisher:  publisher,
		retryBase:  retryBase,
		retryMax:   retryMax,
	}, nil
}

func (d *Dispatcher) DispatchPending(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	events, err := d.repository.ListDispatchableOutboxEvents(ctx, now)
	if err != nil {
		return 0, fmt.Errorf("list dispatchable outbox events: %w", err)
	}

	dispatched := 0
	for _, event := range events {
		nextAttempts := event.Attempts + 1
		if err := d.publisher.Publish(ctx, event); err != nil {
			nextAttemptAt := d.nextRetryAt(now, nextAttempts)
			if markErr := d.repository.UpdateOutboxEventDelivery(ctx, event.DedupeKey, "failed", nextAttempts, &nextAttemptAt); markErr != nil {
				return dispatched, fmt.Errorf("schedule failed outbox event retry: %w", markErr)
			}
			continue
		}

		if err := d.repository.UpdateOutboxEventDelivery(ctx, event.DedupeKey, "published", nextAttempts, nil); err != nil {
			return dispatched, fmt.Errorf("mark published outbox event: %w", err)
		}
		dispatched++
	}

	return dispatched, nil
}

func (d *Dispatcher) nextRetryAt(now time.Time, attempts int) time.Time {
	delay := d.retryBase
	for i := 1; i < attempts; i++ {
		delay *= 2
		if delay >= d.retryMax {
			delay = d.retryMax
			break
		}
	}

	return now.Add(delay)
}

package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
	"github.com/sandeepkv93/go-saga-lab/internal/store"
	"github.com/sandeepkv93/go-saga-lab/internal/telemetry"
)

type Publisher interface {
	Publish(ctx context.Context, event domain.OutboxEvent) error
}

type Dispatcher struct {
	repository store.SagaOutboxRepository
	publisher  Publisher
	backend    string
	retryBase  time.Duration
	retryMax   time.Duration
	leaseOwner string
	leaseTTL   time.Duration
	timeout    time.Duration
}

func NewDispatcher(repository store.SagaOutboxRepository, publisher Publisher, backend string, retryBase, retryMax time.Duration, leaseOwner string, leaseTTL, timeout time.Duration) (*Dispatcher, error) {
	if repository == nil {
		return nil, fmt.Errorf("repository is required")
	}
	if publisher == nil {
		return nil, fmt.Errorf("publisher is required")
	}
	if backend == "" {
		return nil, fmt.Errorf("backend is required")
	}
	if retryBase <= 0 {
		return nil, fmt.Errorf("retry base delay must be positive")
	}
	if retryMax < retryBase {
		return nil, fmt.Errorf("retry max delay must be greater than or equal to retry base delay")
	}
	if leaseOwner == "" {
		return nil, fmt.Errorf("lease owner is required")
	}
	if leaseTTL <= 0 {
		return nil, fmt.Errorf("lease TTL must be positive")
	}
	if timeout <= 0 {
		return nil, fmt.Errorf("publish timeout must be positive")
	}

	return &Dispatcher{
		repository: repository,
		publisher:  publisher,
		backend:    backend,
		retryBase:  retryBase,
		retryMax:   retryMax,
		leaseOwner: leaseOwner,
		leaseTTL:   leaseTTL,
		timeout:    timeout,
	}, nil
}

func (d *Dispatcher) DispatchPending(ctx context.Context) (int, error) {
	now := time.Now().UTC()
	events, err := d.repository.ClaimDispatchableOutboxEvents(ctx, now, d.leaseOwner, now.Add(d.leaseTTL), 100)
	if err != nil {
		return 0, fmt.Errorf("claim dispatchable outbox events: %w", err)
	}

	dispatched := 0
	for _, event := range events {
		nextAttempts := event.Attempts + 1
		publishCtx, cancel := context.WithTimeout(ctx, d.timeout)
		err := d.publisher.Publish(publishCtx, event)
		cancel()
		if err != nil {
			telemetry.RecordOutboxPublish(d.backend, event.EventType, "failed")
			nextAttemptAt := d.nextRetryAt(now, nextAttempts)
			if markErr := d.repository.UpdateOutboxEventDelivery(ctx, event.DedupeKey, "failed", nextAttempts, &nextAttemptAt, d.leaseOwner); markErr != nil {
				return dispatched, fmt.Errorf("schedule failed outbox event retry: %w", markErr)
			}
			continue
		}

		if err := d.repository.UpdateOutboxEventDelivery(ctx, event.DedupeKey, "published", nextAttempts, nil, d.leaseOwner); err != nil {
			return dispatched, fmt.Errorf("mark published outbox event: %w", err)
		}
		telemetry.RecordOutboxPublish(d.backend, event.EventType, "published")
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

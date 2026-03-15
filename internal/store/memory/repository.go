package memory

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
)

var ErrSagaNotFound = errors.New("saga instance not found")

type Repository struct {
	mu           sync.RWMutex
	instances    map[string]domain.SagaInstance
	outboxEvents []domain.OutboxEvent
}

func New() *Repository {
	return &Repository{
		instances: make(map[string]domain.SagaInstance),
	}
}

func (r *Repository) CreateSagaInstance(_ context.Context, instance domain.SagaInstance) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.instances[instance.ID]; exists {
		return errors.New("saga instance already exists")
	}

	r.instances[instance.ID] = instance
	return nil
}

func (r *Repository) GetSagaInstance(_ context.Context, id string) (domain.SagaInstance, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	instance, ok := r.instances[id]
	if !ok {
		return domain.SagaInstance{}, ErrSagaNotFound
	}

	return instance, nil
}

func (r *Repository) UpdateSagaStatus(_ context.Context, id string, status domain.SagaStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	instance, ok := r.instances[id]
	if !ok {
		return ErrSagaNotFound
	}

	instance.Status = status
	instance.UpdatedAt = time.Now().UTC()
	r.instances[id] = instance
	return nil
}

func (r *Repository) CreateSagaInstanceWithOutbox(_ context.Context, instance domain.SagaInstance, event domain.OutboxEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.instances[instance.ID]; exists {
		return errors.New("saga instance already exists")
	}

	r.instances[instance.ID] = instance
	r.outboxEvents = append(r.outboxEvents, event)
	return nil
}

func (r *Repository) ClaimDispatchableOutboxEvents(_ context.Context, now time.Time, leaseOwner string, leaseUntil time.Time, limit int) ([]domain.OutboxEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if leaseOwner == "" {
		return nil, errors.New("lease owner is required")
	}
	if limit <= 0 {
		limit = 100
	}

	events := make([]domain.OutboxEvent, 0, len(r.outboxEvents))
	for i := range r.outboxEvents {
		event := &r.outboxEvents[i]
		leasedElsewhere := event.LeaseUntil != nil && event.LeaseOwner != "" && event.LeaseOwner != leaseOwner && event.LeaseUntil.After(now)
		if leasedElsewhere {
			continue
		}
		dispatchable := event.Status == "pending" || (event.Status == "failed" && event.NextAttemptAt != nil && !event.NextAttemptAt.After(now))
		if !dispatchable {
			continue
		}

		event.LeaseOwner = leaseOwner
		event.LeaseUntil = &leaseUntil
		event.UpdatedAt = now
		events = append(events, *event)
		if len(events) == limit {
			break
		}
	}

	return events, nil
}

func (r *Repository) UpdateOutboxEventDelivery(_ context.Context, dedupeKey string, status string, attempts int, nextAttemptAt *time.Time, leaseOwner string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.outboxEvents {
		if r.outboxEvents[i].DedupeKey != dedupeKey {
			continue
		}
		if leaseOwner != "" && r.outboxEvents[i].LeaseOwner != leaseOwner {
			return errors.New("outbox event lease owner mismatch")
		}
		r.outboxEvents[i].Status = status
		r.outboxEvents[i].Attempts = attempts
		r.outboxEvents[i].UpdatedAt = time.Now().UTC()
		r.outboxEvents[i].NextAttemptAt = nextAttemptAt
		r.outboxEvents[i].LeaseOwner = ""
		r.outboxEvents[i].LeaseUntil = nil
		return nil
	}

	return ErrSagaNotFound
}

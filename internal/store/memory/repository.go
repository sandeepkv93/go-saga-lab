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

func (r *Repository) ListPendingOutboxEvents(_ context.Context) ([]domain.OutboxEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	events := make([]domain.OutboxEvent, 0, len(r.outboxEvents))
	for _, event := range r.outboxEvents {
		if event.Status == "pending" {
			events = append(events, event)
		}
	}
	return events, nil
}

func (r *Repository) MarkOutboxEventStatus(_ context.Context, dedupeKey string, status string, attempts int) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.outboxEvents {
		if r.outboxEvents[i].DedupeKey != dedupeKey {
			continue
		}
		r.outboxEvents[i].Status = status
		r.outboxEvents[i].Attempts = attempts
		r.outboxEvents[i].UpdatedAt = time.Now().UTC()
		return nil
	}

	return ErrSagaNotFound
}

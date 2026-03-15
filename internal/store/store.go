package store

import (
	"context"
	"time"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
)

type SagaRepository interface {
	CreateSagaInstance(ctx context.Context, instance domain.SagaInstance) error
	GetSagaInstance(ctx context.Context, id string) (domain.SagaInstance, error)
	UpdateSagaStatus(ctx context.Context, id string, status domain.SagaStatus) error
}

type SagaOutboxRepository interface {
	SagaRepository
	CreateSagaInstanceWithOutbox(ctx context.Context, instance domain.SagaInstance, event domain.OutboxEvent) error
	ListDispatchableOutboxEvents(ctx context.Context, now time.Time) ([]domain.OutboxEvent, error)
	UpdateOutboxEventDelivery(ctx context.Context, dedupeKey string, status string, attempts int, nextAttemptAt *time.Time) error
}

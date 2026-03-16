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
	CreateStepExecutions(ctx context.Context, sagaID string, steps []domain.StepExecution) error
	ListStepExecutions(ctx context.Context, sagaID string) ([]domain.StepExecution, error)
	UpdateStepExecution(ctx context.Context, sagaID, stepName string, status domain.StepExecutionStatus, attempts int, lastError string, finishedAt *time.Time) error
}

type SagaOutboxRepository interface {
	SagaRepository
	CreateSagaInstanceWithOutbox(ctx context.Context, instance domain.SagaInstance, event domain.OutboxEvent) error
	ClaimDispatchableOutboxEvents(ctx context.Context, now time.Time, leaseOwner string, leaseUntil time.Time, limit int) ([]domain.OutboxEvent, error)
	UpdateOutboxEventDelivery(ctx context.Context, dedupeKey string, status string, attempts int, nextAttemptAt *time.Time, leaseOwner string) error
}

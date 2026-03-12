package runtime

import (
	"context"
	"fmt"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
	"github.com/sandeepkv93/go-saga-lab/internal/orchestrator/statemachine"
	"github.com/sandeepkv93/go-saga-lab/internal/store"
)

type Service struct {
	repository store.SagaRepository
}

func NewService(repository store.SagaRepository) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("repository is required")
	}

	return &Service{repository: repository}, nil
}

func (s *Service) StartSaga(ctx context.Context, sagaID string) (domain.SagaStatus, error) {
	return s.transition(ctx, sagaID, domain.EventStart)
}

func (s *Service) HandleStepResult(ctx context.Context, sagaID string, succeeded bool) (domain.SagaStatus, error) {
	event := domain.EventStepFailed
	if succeeded {
		event = domain.EventStepSucceeded
	}

	return s.transition(ctx, sagaID, event)
}

func (s *Service) CompleteCompensation(ctx context.Context, sagaID string, succeeded bool) (domain.SagaStatus, error) {
	event := domain.EventCompensationFault
	if succeeded {
		event = domain.EventCompensationDone
	}

	return s.transition(ctx, sagaID, event)
}

func (s *Service) CancelSaga(ctx context.Context, sagaID string) (domain.SagaStatus, error) {
	return s.transition(ctx, sagaID, domain.EventCancel)
}

func (s *Service) transition(ctx context.Context, sagaID string, event domain.SagaEvent) (domain.SagaStatus, error) {
	instance, err := s.repository.GetSagaInstance(ctx, sagaID)
	if err != nil {
		return "", fmt.Errorf("load saga instance: %w", err)
	}

	next, err := statemachine.NextStatus(instance.Status, event)
	if err != nil {
		return "", fmt.Errorf("compute next saga status: %w", err)
	}

	if err := s.repository.UpdateSagaStatus(ctx, sagaID, next); err != nil {
		return "", fmt.Errorf("persist next saga status: %w", err)
	}

	return next, nil
}

package runtime

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
)

type fakeSagaRepository struct {
	instances map[string]domain.SagaInstance
}

func newFakeSagaRepository(instances ...domain.SagaInstance) *fakeSagaRepository {
	repo := &fakeSagaRepository{instances: make(map[string]domain.SagaInstance, len(instances))}
	for _, instance := range instances {
		repo.instances[instance.ID] = instance
	}
	return repo
}

func (r *fakeSagaRepository) CreateSagaInstance(_ context.Context, instance domain.SagaInstance) error {
	r.instances[instance.ID] = instance
	return nil
}

func (r *fakeSagaRepository) GetSagaInstance(_ context.Context, id string) (domain.SagaInstance, error) {
	instance, ok := r.instances[id]
	if !ok {
		return domain.SagaInstance{}, errors.New("not found")
	}
	return instance, nil
}

func (r *fakeSagaRepository) UpdateSagaStatus(_ context.Context, id string, status domain.SagaStatus) error {
	instance, ok := r.instances[id]
	if !ok {
		return errors.New("not found")
	}
	instance.Status = status
	instance.UpdatedAt = time.Now().UTC()
	r.instances[id] = instance
	return nil
}

func TestNewServiceRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := NewService(nil)
	if err == nil {
		t.Fatal("expected error for nil repository")
	}
}

func TestServiceStartSaga(t *testing.T) {
	t.Parallel()

	repo := newFakeSagaRepository(domain.SagaInstance{
		ID:         "saga-1",
		TemplateID: "order",
		Status:     domain.SagaStatusCreated,
	})

	service, err := NewService(repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	got, err := service.StartSaga(context.Background(), "saga-1")
	if err != nil {
		t.Fatalf("StartSaga() error = %v", err)
	}
	if got != domain.SagaStatusRunning {
		t.Fatalf("StartSaga() = %q, want %q", got, domain.SagaStatusRunning)
	}
}

func TestServiceTransitionsThroughFailurePath(t *testing.T) {
	t.Parallel()

	repo := newFakeSagaRepository(domain.SagaInstance{
		ID:         "saga-2",
		TemplateID: "order",
		Status:     domain.SagaStatusCreated,
	})

	service, err := NewService(repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if _, err := service.StartSaga(context.Background(), "saga-2"); err != nil {
		t.Fatalf("StartSaga() error = %v", err)
	}

	got, err := service.HandleStepResult(context.Background(), "saga-2", false)
	if err != nil {
		t.Fatalf("HandleStepResult(false) error = %v", err)
	}
	if got != domain.SagaStatusCompensating {
		t.Fatalf("HandleStepResult(false) = %q, want %q", got, domain.SagaStatusCompensating)
	}

	got, err = service.CompleteCompensation(context.Background(), "saga-2", true)
	if err != nil {
		t.Fatalf("CompleteCompensation(true) error = %v", err)
	}
	if got != domain.SagaStatusCancelled {
		t.Fatalf("CompleteCompensation(true) = %q, want %q", got, domain.SagaStatusCancelled)
	}
}

func TestServiceRejectsIllegalTransition(t *testing.T) {
	t.Parallel()

	repo := newFakeSagaRepository(domain.SagaInstance{
		ID:         "saga-3",
		TemplateID: "order",
		Status:     domain.SagaStatusCompleted,
	})

	service, err := NewService(repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	if _, err := service.CancelSaga(context.Background(), "saga-3"); err == nil {
		t.Fatal("expected error for cancelling a completed saga")
	}
}

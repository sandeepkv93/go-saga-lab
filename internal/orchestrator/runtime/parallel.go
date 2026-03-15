package runtime

import (
	"fmt"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
)

type ParallelExecution struct {
	steps map[string]domain.StepExecutionStatus
}

func NewParallelExecution(stepNames []string) (*ParallelExecution, error) {
	if len(stepNames) == 0 {
		return nil, fmt.Errorf("at least one step is required")
	}

	steps := make(map[string]domain.StepExecutionStatus, len(stepNames))
	for _, stepName := range stepNames {
		if stepName == "" {
			return nil, fmt.Errorf("step name is required")
		}
		if _, exists := steps[stepName]; exists {
			return nil, fmt.Errorf("duplicate step name: %q", stepName)
		}
		steps[stepName] = domain.StepStatusPending
	}

	return &ParallelExecution{steps: steps}, nil
}

func (p *ParallelExecution) StartAll() {
	for stepName, status := range p.steps {
		if status == domain.StepStatusPending {
			p.steps[stepName] = domain.StepStatusInProgress
		}
	}
}

func (p *ParallelExecution) RecordStepResult(stepName string, succeeded bool) (domain.SagaStatus, error) {
	status, ok := p.steps[stepName]
	if !ok {
		return "", fmt.Errorf("unknown step: %q", stepName)
	}
	if status == domain.StepStatusSucceeded || status == domain.StepStatusFailed {
		return "", fmt.Errorf("step %q already completed", stepName)
	}

	if succeeded {
		p.steps[stepName] = domain.StepStatusSucceeded
	} else {
		p.steps[stepName] = domain.StepStatusFailed
		return domain.SagaStatusCompensating, nil
	}

	for _, stepStatus := range p.steps {
		if stepStatus != domain.StepStatusSucceeded {
			return domain.SagaStatusRunning, nil
		}
	}

	return domain.SagaStatusCompleted, nil
}

func (p *ParallelExecution) Snapshot() map[string]domain.StepExecutionStatus {
	snapshot := make(map[string]domain.StepExecutionStatus, len(p.steps))
	for stepName, status := range p.steps {
		snapshot[stepName] = status
	}
	return snapshot
}

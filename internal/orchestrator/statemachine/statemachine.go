package statemachine

import (
	"fmt"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
)

var transitions = map[domain.SagaStatus]map[domain.SagaEvent]domain.SagaStatus{
	domain.SagaStatusCreated: {
		domain.EventStart:  domain.SagaStatusRunning,
		domain.EventCancel: domain.SagaStatusCancelled,
	},
	domain.SagaStatusRunning: {
		domain.EventStepSucceeded: domain.SagaStatusCompleted,
		domain.EventStepFailed:    domain.SagaStatusCompensating,
		domain.EventCancel:        domain.SagaStatusCompensating,
	},
	domain.SagaStatusCompensating: {
		domain.EventCompensationDone:  domain.SagaStatusCancelled,
		domain.EventCompensationFault: domain.SagaStatusFailed,
	},
}

func NextStatus(current domain.SagaStatus, event domain.SagaEvent) (domain.SagaStatus, error) {
	if err := domain.ValidateTransitionInput(current, event); err != nil {
		return "", err
	}

	allowedEvents, ok := transitions[current]
	if !ok {
		return "", fmt.Errorf("no transitions configured for status %q", current)
	}

	next, ok := allowedEvents[event]
	if !ok {
		return "", fmt.Errorf("illegal transition: status=%q event=%q", current, event)
	}

	return next, nil
}

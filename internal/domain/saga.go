package domain

import (
	"fmt"
	"time"
)

type SagaStatus string

const (
	SagaStatusCreated      SagaStatus = "created"
	SagaStatusRunning      SagaStatus = "running"
	SagaStatusCompensating SagaStatus = "compensating"
	SagaStatusCompleted    SagaStatus = "completed"
	SagaStatusFailed       SagaStatus = "failed"
	SagaStatusCancelled    SagaStatus = "cancelled"
)

type SagaEvent string

const (
	EventStart             SagaEvent = "start"
	EventStepSucceeded     SagaEvent = "step_succeeded"
	EventStepFailed        SagaEvent = "step_failed"
	EventCompensationDone  SagaEvent = "compensation_succeeded"
	EventCompensationFault SagaEvent = "compensation_failed"
	EventCancel            SagaEvent = "cancel"
)

type SagaInstance struct {
	ID             string
	TemplateID     string
	Status         SagaStatus
	InputJSON      []byte
	IdempotencyKey string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type OutboxEvent struct {
	AggregateType string
	AggregateID   string
	EventType     string
	PayloadJSON   []byte
	DedupeKey     string
	Status        string
	Attempts      int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	NextAttemptAt *time.Time
}

func (s SagaStatus) Valid() bool {
	switch s {
	case SagaStatusCreated, SagaStatusRunning, SagaStatusCompensating, SagaStatusCompleted, SagaStatusFailed, SagaStatusCancelled:
		return true
	default:
		return false
	}
}

func (e SagaEvent) Valid() bool {
	switch e {
	case EventStart, EventStepSucceeded, EventStepFailed, EventCompensationDone, EventCompensationFault, EventCancel:
		return true
	default:
		return false
	}
}

func ValidateTransitionInput(status SagaStatus, event SagaEvent) error {
	if !status.Valid() {
		return fmt.Errorf("invalid status: %q", status)
	}
	if !event.Valid() {
		return fmt.Errorf("invalid event: %q", event)
	}
	return nil
}

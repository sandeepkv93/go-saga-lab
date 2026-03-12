package statemachine

import (
	"testing"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
)

func TestNextStatus_AllowsConfiguredTransitions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		current domain.SagaStatus
		event   domain.SagaEvent
		want    domain.SagaStatus
	}{
		{
			name:    "created to running",
			current: domain.SagaStatusCreated,
			event:   domain.EventStart,
			want:    domain.SagaStatusRunning,
		},
		{
			name:    "created to cancelled",
			current: domain.SagaStatusCreated,
			event:   domain.EventCancel,
			want:    domain.SagaStatusCancelled,
		},
		{
			name:    "running to completed",
			current: domain.SagaStatusRunning,
			event:   domain.EventStepSucceeded,
			want:    domain.SagaStatusCompleted,
		},
		{
			name:    "running to compensating on failure",
			current: domain.SagaStatusRunning,
			event:   domain.EventStepFailed,
			want:    domain.SagaStatusCompensating,
		},
		{
			name:    "compensating to failed",
			current: domain.SagaStatusCompensating,
			event:   domain.EventCompensationFault,
			want:    domain.SagaStatusFailed,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := NextStatus(tc.current, tc.event)
			if err != nil {
				t.Fatalf("NextStatus(%q, %q) returned error: %v", tc.current, tc.event, err)
			}
			if got != tc.want {
				t.Fatalf("NextStatus(%q, %q) = %q, want %q", tc.current, tc.event, got, tc.want)
			}
		})
	}
}

func TestNextStatus_RejectsIllegalTransitions(t *testing.T) {
	t.Parallel()

	_, err := NextStatus(domain.SagaStatusCompleted, domain.EventCancel)
	if err == nil {
		t.Fatal("expected error for illegal transition from completed")
	}
}

func TestNextStatus_RejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	_, err := NextStatus(domain.SagaStatus("bogus"), domain.EventStart)
	if err == nil {
		t.Fatal("expected error for invalid status")
	}

	_, err = NextStatus(domain.SagaStatusCreated, domain.SagaEvent("bogus"))
	if err == nil {
		t.Fatal("expected error for invalid event")
	}
}

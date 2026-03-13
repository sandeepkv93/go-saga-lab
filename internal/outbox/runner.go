package outbox

import (
	"context"
	"fmt"
	"log"
	"time"
)

type Runner struct {
	dispatcher   *Dispatcher
	pollInterval time.Duration
	runOnce      bool
}

func NewRunner(dispatcher *Dispatcher, pollInterval time.Duration, runOnce bool) (*Runner, error) {
	if dispatcher == nil {
		return nil, fmt.Errorf("dispatcher is required")
	}
	if pollInterval <= 0 {
		return nil, fmt.Errorf("poll interval must be positive")
	}

	return &Runner{
		dispatcher:   dispatcher,
		pollInterval: pollInterval,
		runOnce:      runOnce,
	}, nil
}

func (r *Runner) Run(ctx context.Context) error {
	if r.runOnce {
		_, err := r.dispatcher.DispatchPending(ctx)
		return err
	}

	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()

	for {
		dispatched, err := r.dispatcher.DispatchPending(ctx)
		if err != nil {
			return err
		}
		if dispatched > 0 {
			log.Printf("publisher dispatched %d outbox event(s)", dispatched)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

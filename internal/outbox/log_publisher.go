package outbox

import (
	"context"
	"log"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
)

type LogPublisher struct{}

func (p *LogPublisher) Publish(_ context.Context, event domain.OutboxEvent) error {
	log.Printf(
		"published outbox event aggregate_type=%s aggregate_id=%s event_type=%s dedupe_key=%s",
		event.AggregateType,
		event.AggregateID,
		event.EventType,
		event.DedupeKey,
	)
	return nil
}

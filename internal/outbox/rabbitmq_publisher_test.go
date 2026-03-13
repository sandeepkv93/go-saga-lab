package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
)

type fakeAMQPChannel struct {
	exchange   string
	routingKey string
	message    amqp.Publishing
	err        error
}

func (c *fakeAMQPChannel) PublishWithContext(_ context.Context, exchange, key string, _ bool, _ bool, msg amqp.Publishing) error {
	c.exchange = exchange
	c.routingKey = key
	c.message = msg
	return c.err
}

func (c *fakeAMQPChannel) Close() error {
	return nil
}

type fakeAMQPConnection struct{}

func (c *fakeAMQPConnection) Channel() (amqpChannel, error) {
	return nil, errors.New("not used")
}

func (c *fakeAMQPConnection) Close() error {
	return nil
}

func TestRabbitMQPublisherPublish(t *testing.T) {
	t.Parallel()

	channel := &fakeAMQPChannel{}
	publisher, err := newRabbitMQPublisherFromConnection(&fakeAMQPConnection{}, channel, "go_saga_lab.events", "saga")
	if err != nil {
		t.Fatalf("newRabbitMQPublisherFromConnection() error = %v", err)
	}

	event := domain.OutboxEvent{
		AggregateType: "saga",
		AggregateID:   "saga-1",
		EventType:     "created",
		PayloadJSON:   []byte(`{"saga_id":"saga-1"}`),
		DedupeKey:     "dedupe-1",
	}
	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	if channel.exchange != "go_saga_lab.events" {
		t.Fatalf("exchange = %q, want %q", channel.exchange, "go_saga_lab.events")
	}
	if channel.routingKey != "saga.created" {
		t.Fatalf("routingKey = %q, want %q", channel.routingKey, "saga.created")
	}
	if channel.message.MessageId != "dedupe-1" {
		t.Fatalf("MessageId = %q, want %q", channel.message.MessageId, "dedupe-1")
	}

	var payload map[string]any
	if err := json.Unmarshal(channel.message.Body, &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload["event_type"] != "created" {
		t.Fatalf("payload[event_type] = %v, want %q", payload["event_type"], "created")
	}
}

func TestRabbitMQPublisherRequiresSettings(t *testing.T) {
	t.Parallel()

	channel := &fakeAMQPChannel{}
	if _, err := newRabbitMQPublisherFromConnection(nil, channel, "x", "saga"); err == nil {
		t.Fatal("expected error for nil connection")
	}
	if _, err := newRabbitMQPublisherFromConnection(&fakeAMQPConnection{}, nil, "x", "saga"); err == nil {
		t.Fatal("expected error for nil channel")
	}
	if _, err := newRabbitMQPublisherFromConnection(&fakeAMQPConnection{}, channel, "", "saga"); err == nil {
		t.Fatal("expected error for empty exchange")
	}
	if _, err := newRabbitMQPublisherFromConnection(&fakeAMQPConnection{}, channel, "x", ""); err == nil {
		t.Fatal("expected error for empty routing key prefix")
	}
}

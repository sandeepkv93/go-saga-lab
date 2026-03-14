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
	exchange         string
	exchangeType     string
	queue            string
	bindingKey       string
	routingKey       string
	message          amqp.Publishing
	publishErr       error
	exchangeDeclErr  error
	queueDeclareErr  error
	queueBindErr     error
	topologyDeclared bool
}

func (c *fakeAMQPChannel) ExchangeDeclare(name, kind string, _ bool, _ bool, _ bool, _ bool, _ amqp.Table) error {
	c.exchange = name
	c.exchangeType = kind
	c.topologyDeclared = true
	return c.exchangeDeclErr
}

func (c *fakeAMQPChannel) QueueDeclare(name string, _ bool, _ bool, _ bool, _ bool, _ amqp.Table) (amqp.Queue, error) {
	c.queue = name
	return amqp.Queue{Name: name}, c.queueDeclareErr
}

func (c *fakeAMQPChannel) QueueBind(name, key, exchange string, _ bool, _ amqp.Table) error {
	c.queue = name
	c.bindingKey = key
	c.exchange = exchange
	return c.queueBindErr
}

func (c *fakeAMQPChannel) PublishWithContext(_ context.Context, exchange, key string, _ bool, _ bool, msg amqp.Publishing) error {
	c.exchange = exchange
	c.routingKey = key
	c.message = msg
	return c.publishErr
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
	publisher, err := newRabbitMQPublisherFromConnection(&fakeAMQPConnection{}, channel, "go_saga_lab.events", "topic", "go_saga_lab.events.demo", "saga")
	if err != nil {
		t.Fatalf("newRabbitMQPublisherFromConnection() error = %v", err)
	}
	if !channel.topologyDeclared {
		t.Fatal("expected topology to be declared")
	}
	if channel.exchangeType != "topic" {
		t.Fatalf("exchangeType = %q, want %q", channel.exchangeType, "topic")
	}
	if channel.queue != "go_saga_lab.events.demo" {
		t.Fatalf("queue = %q, want %q", channel.queue, "go_saga_lab.events.demo")
	}
	if channel.bindingKey != "saga.*" {
		t.Fatalf("bindingKey = %q, want %q", channel.bindingKey, "saga.*")
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
	if _, err := newRabbitMQPublisherFromConnection(nil, channel, "x", "topic", "queue", "saga"); err == nil {
		t.Fatal("expected error for nil connection")
	}
	if _, err := newRabbitMQPublisherFromConnection(&fakeAMQPConnection{}, nil, "x", "topic", "queue", "saga"); err == nil {
		t.Fatal("expected error for nil channel")
	}
	if _, err := newRabbitMQPublisherFromConnection(&fakeAMQPConnection{}, channel, "", "topic", "queue", "saga"); err == nil {
		t.Fatal("expected error for empty exchange")
	}
	if _, err := newRabbitMQPublisherFromConnection(&fakeAMQPConnection{}, channel, "x", "", "queue", "saga"); err == nil {
		t.Fatal("expected error for empty exchange type")
	}
	if _, err := newRabbitMQPublisherFromConnection(&fakeAMQPConnection{}, channel, "x", "topic", "", "saga"); err == nil {
		t.Fatal("expected error for empty queue")
	}
	if _, err := newRabbitMQPublisherFromConnection(&fakeAMQPConnection{}, channel, "x", "topic", "queue", ""); err == nil {
		t.Fatal("expected error for empty routing key prefix")
	}
}

func TestRabbitMQPublisherFailsWhenTopologySetupFails(t *testing.T) {
	t.Parallel()

	channel := &fakeAMQPChannel{queueBindErr: errors.New("bind failed")}
	if _, err := newRabbitMQPublisherFromConnection(&fakeAMQPConnection{}, channel, "x", "topic", "queue", "saga"); err == nil {
		t.Fatal("expected topology setup error")
	}
}

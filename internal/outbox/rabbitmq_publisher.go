package outbox

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
)

type amqpChannel interface {
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	Close() error
}

type amqpConnection interface {
	Channel() (amqpChannel, error)
	Close() error
}

type rabbitMQConnection struct {
	conn *amqp.Connection
}

func (c *rabbitMQConnection) Channel() (amqpChannel, error) {
	return c.conn.Channel()
}

func (c *rabbitMQConnection) Close() error {
	return c.conn.Close()
}

type RabbitMQPublisher struct {
	connection       amqpConnection
	channel          amqpChannel
	exchange         string
	exchangeType     string
	queue            string
	routingKeyPrefix string
}

func NewRabbitMQPublisher(amqpURL, exchange, exchangeType, queue, routingKeyPrefix string) (*RabbitMQPublisher, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}

	return newRabbitMQPublisherFromConnection(&rabbitMQConnection{conn: conn}, channel, exchange, exchangeType, queue, routingKeyPrefix)
}

func newRabbitMQPublisherFromConnection(connection amqpConnection, channel amqpChannel, exchange, exchangeType, queue, routingKeyPrefix string) (*RabbitMQPublisher, error) {
	if connection == nil {
		return nil, fmt.Errorf("connection is required")
	}
	if channel == nil {
		return nil, fmt.Errorf("channel is required")
	}
	if exchange == "" {
		return nil, fmt.Errorf("exchange is required")
	}
	if exchangeType == "" {
		return nil, fmt.Errorf("exchange type is required")
	}
	if queue == "" {
		return nil, fmt.Errorf("queue is required")
	}
	if routingKeyPrefix == "" {
		return nil, fmt.Errorf("routing key prefix is required")
	}

	publisher := &RabbitMQPublisher{
		connection:       connection,
		channel:          channel,
		exchange:         exchange,
		exchangeType:     exchangeType,
		queue:            queue,
		routingKeyPrefix: routingKeyPrefix,
	}

	if err := publisher.ensureTopology(); err != nil {
		return nil, err
	}

	return publisher, nil
}

func (p *RabbitMQPublisher) Publish(ctx context.Context, event domain.OutboxEvent) error {
	body, err := json.Marshal(map[string]any{
		"aggregate_type": event.AggregateType,
		"aggregate_id":   event.AggregateID,
		"event_type":     event.EventType,
		"dedupe_key":     event.DedupeKey,
		"payload":        json.RawMessage(event.PayloadJSON),
	})
	if err != nil {
		return fmt.Errorf("marshal outbox event: %w", err)
	}

	routingKey := fmt.Sprintf("%s.%s", p.routingKeyPrefix, event.EventType)
	if err := p.channel.PublishWithContext(
		ctx,
		p.exchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Type:         event.EventType,
			MessageId:    event.DedupeKey,
			Body:         body,
		},
	); err != nil {
		return fmt.Errorf("publish rabbitmq message: %w", err)
	}

	return nil
}

func (p *RabbitMQPublisher) Close() error {
	var closeErr error
	if p.channel != nil {
		if err := p.channel.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	if p.connection != nil {
		if err := p.connection.Close(); err != nil && closeErr == nil {
			closeErr = err
		}
	}
	return closeErr
}

func (p *RabbitMQPublisher) ensureTopology() error {
	if err := p.channel.ExchangeDeclare(p.exchange, p.exchangeType, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}
	if _, err := p.channel.QueueDeclare(p.queue, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare queue: %w", err)
	}
	bindingKey := fmt.Sprintf("%s.*", p.routingKeyPrefix)
	if err := p.channel.QueueBind(p.queue, bindingKey, p.exchange, false, nil); err != nil {
		return fmt.Errorf("bind queue: %w", err)
	}

	return nil
}

package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/sandeepkv93/go-saga-lab/internal/config"
	"github.com/sandeepkv93/go-saga-lab/internal/outbox"
	"github.com/sandeepkv93/go-saga-lab/internal/store"
	"github.com/sandeepkv93/go-saga-lab/internal/store/memory"
	"github.com/sandeepkv93/go-saga-lab/internal/store/postgres"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	repository, cleanup, err := buildOutboxRepository(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	publisher, cleanupPublisher, err := buildPublisher(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if cleanupPublisher != nil {
		defer cleanupPublisher()
	}

	dispatcher, err := outbox.NewDispatcher(repository, publisher, cfg.PublisherRetryBase, cfg.PublisherRetryMax)
	if err != nil {
		log.Fatal(err)
	}
	runner, err := outbox.NewRunner(dispatcher, cfg.PublisherPollInterval, cfg.PublisherRunOnce)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf(
		"go-saga-lab publisher starting backend=%s poll_interval=%s retry_base=%s retry_max=%s run_once=%t",
		cfg.PublisherBackend,
		cfg.PublisherPollInterval,
		cfg.PublisherRetryBase,
		cfg.PublisherRetryMax,
		cfg.PublisherRunOnce,
	)
	if err := runner.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
}

func buildPublisher(cfg config.Config) (outbox.Publisher, func() error, error) {
	switch cfg.PublisherBackend {
	case "log":
		return &outbox.LogPublisher{}, nil, nil
	case "rabbitmq":
		publisher, err := outbox.NewRabbitMQPublisher(
			cfg.AMQPURL,
			cfg.AMQPExchange,
			cfg.AMQPExchangeType,
			cfg.AMQPQueue,
			cfg.AMQPRoutingKeyPrefix,
		)
		if err != nil {
			return nil, nil, err
		}
		return publisher, publisher.Close, nil
	default:
		return nil, nil, http.ErrNotSupported
	}
}

func buildOutboxRepository(ctx context.Context, cfg config.Config) (store.SagaOutboxRepository, func(), error) {
	switch cfg.StorageBackend {
	case "memory":
		return memory.New(), nil, nil
	case "postgres":
		repository, err := postgres.New(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, nil, err
		}
		if err := repository.Ping(ctx); err != nil {
			repository.Close()
			return nil, nil, err
		}
		if err := repository.Migrate(ctx, cfg.MigrationsDir); err != nil {
			repository.Close()
			return nil, nil, err
		}
		return repository, repository.Close, nil
	default:
		return nil, nil, http.ErrNotSupported
	}
}

package main

import (
	"context"
	"log"
	"net/http"

	httpapi "github.com/sandeepkv93/go-saga-lab/internal/api/httpapi"
	"github.com/sandeepkv93/go-saga-lab/internal/config"
	"github.com/sandeepkv93/go-saga-lab/internal/store"
	"github.com/sandeepkv93/go-saga-lab/internal/store/memory"
	"github.com/sandeepkv93/go-saga-lab/internal/store/postgres"
)

func main() {
	cfg := config.Load()

	repository, cleanup, err := buildRepository(context.Background(), cfg)
	if err != nil {
		log.Fatal(err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	server, err := httpapi.NewDefaultServer(context.Background(), repository)
	if err != nil {
		log.Fatal(err)
	}

	addr := ":" + cfg.Port

	log.Printf("go-saga-lab api listening on %s", addr)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		log.Fatal(err)
	}
}

func buildRepository(ctx context.Context, cfg config.Config) (store.SagaRepository, func(), error) {
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

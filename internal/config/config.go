package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                  string
	StorageBackend        string
	DatabaseURL           string
	MigrationsDir         string
	PublisherPollInterval time.Duration
	PublisherRunOnce      bool
	PublisherBackend      string
	AMQPURL               string
	AMQPExchange          string
	AMQPRoutingKeyPrefix  string
}

func Load() Config {
	cfg := Config{
		Port:                  getEnv("PORT", "8080"),
		StorageBackend:        getEnv("STORAGE_BACKEND", "memory"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		MigrationsDir:         getEnv("MIGRATIONS_DIR", "migrations"),
		PublisherPollInterval: getEnvDurationFromMilliseconds("PUBLISHER_POLL_INTERVAL_MS", 2000),
		PublisherRunOnce:      getEnvBool("PUBLISHER_RUN_ONCE", false),
		PublisherBackend:      getEnv("PUBLISHER_BACKEND", "log"),
		AMQPURL:               getEnv("AMQP_URL", "amqp://guest:guest@localhost:5672/"),
		AMQPExchange:          getEnv("AMQP_EXCHANGE", "go_saga_lab.events"),
		AMQPRoutingKeyPrefix:  getEnv("AMQP_ROUTING_KEY_PREFIX", "saga"),
	}

	if cfg.DatabaseURL != "" && cfg.StorageBackend == "memory" {
		cfg.StorageBackend = "postgres"
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvDurationFromMilliseconds(key string, fallbackMS int) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return time.Duration(fallbackMS) * time.Millisecond
	}

	ms, err := strconv.Atoi(value)
	if err != nil || ms <= 0 {
		return time.Duration(fallbackMS) * time.Millisecond
	}

	return time.Duration(ms) * time.Millisecond
}

func getEnvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}

	return parsed
}

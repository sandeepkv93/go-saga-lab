package config

import "os"

type Config struct {
	Port           string
	StorageBackend string
	DatabaseURL    string
	MigrationsDir  string
}

func Load() Config {
	cfg := Config{
		Port:           getEnv("PORT", "8080"),
		StorageBackend: getEnv("STORAGE_BACKEND", "memory"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		MigrationsDir:  getEnv("MIGRATIONS_DIR", "migrations"),
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

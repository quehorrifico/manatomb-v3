package config

import (
	"log"
	"os"
)

type Config struct {
	DatabaseURL   string
	Port          string
	SessionSecret string
}

func Load() *Config {
	cfg := &Config{
		DatabaseURL:   mustEnv("DATABASE_URL"),
		Port:          getEnv("PORT", "8080"),
		SessionSecret: mustEnv("SESSION_SECRET"),
	}
	return cfg
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("missing required env var: %s", key)
	}
	return v
}

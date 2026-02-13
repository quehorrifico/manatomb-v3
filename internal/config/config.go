package config

import (
	"log"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL         string
	Port                string
	SessionSecret       string
	SessionCookieSecure bool
	CardSyncOn          bool
	CardSyncMaxRows     int
}

func Load() *Config {
	cfg := &Config{
		DatabaseURL:         mustEnv("DATABASE_URL"),
		Port:                getEnv("PORT", "8080"),
		SessionSecret:       mustEnv("SESSION_SECRET"),
		SessionCookieSecure: getEnvBool("SESSION_COOKIE_SECURE", false),
		CardSyncOn:          getEnvBool("CARD_SYNC_ENABLED", true),
		CardSyncMaxRows:     getEnvInt("CARD_SYNC_MAX_ROWS", 0),
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

func getEnvBool(key string, def bool) bool {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		log.Printf("invalid bool for %s=%q; using default %t", key, raw, def)
		return def
	}
	return value
}

func getEnvInt(key string, def int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return def
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("invalid int for %s=%q; using default %d", key, raw, def)
		return def
	}
	if value < 0 {
		log.Printf("invalid negative int for %s=%q; using default %d", key, raw, def)
		return def
	}
	return value
}

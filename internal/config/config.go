package config

import (
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	DatabaseURL         string
	Port                string
	SessionSecret       string
	SessionCookieSecure bool
	PublicBaseURL       string
	SMTPHost            string
	SMTPPort            string
	SMTPUsername        string
	SMTPPassword        string
	SMTPFrom            string
	TrustedProxyHops    int
	CardSyncOn          bool
	CardSyncOnStart     bool
	CardSyncMaxRows     int
}

func Load() *Config {
	loadDotEnv(".env")

	cfg := &Config{
		DatabaseURL:         mustEnv("DATABASE_URL"),
		Port:                getEnv("PORT", "8080"),
		SessionSecret:       mustEnv("SESSION_SECRET"),
		SessionCookieSecure: getEnvBool("SESSION_COOKIE_SECURE", false),
		PublicBaseURL:       strings.TrimRight(getEnv("PUBLIC_BASE_URL", ""), "/"),
		SMTPHost:            getEnv("SMTP_HOST", ""),
		SMTPPort:            getEnv("SMTP_PORT", "587"),
		SMTPUsername:        getEnv("SMTP_USERNAME", ""),
		SMTPPassword:        getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:            getEnv("SMTP_FROM", ""),
		TrustedProxyHops:    getEnvInt("TRUSTED_PROXY_HOPS", 0),
		CardSyncOn:          getEnvBool("CARD_SYNC_ENABLED", true),
		CardSyncOnStart:     getEnvBool("CARD_SYNC_ON_START", false),
		CardSyncMaxRows:     getEnvInt("CARD_SYNC_MAX_ROWS", 0),
	}
	if cfg.PublicBaseURL == "" {
		cfg.PublicBaseURL = "http://localhost:" + cfg.Port
	}
	publicURL, err := url.Parse(cfg.PublicBaseURL)
	if err != nil ||
		publicURL.Host == "" ||
		(publicURL.Scheme != "http" && publicURL.Scheme != "https") ||
		(publicURL.Path != "" && publicURL.Path != "/") ||
		publicURL.RawQuery != "" ||
		publicURL.Fragment != "" ||
		publicURL.User != nil {
		log.Fatalf("PUBLIC_BASE_URL must be an absolute http or https origin without a path, query, or credentials")
	}
	if publicURL.Scheme == "https" {
		cfg.SessionCookieSecure = true
	}
	return cfg
}

func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if os.Getenv(key) != "" {
			continue
		}
		_ = os.Setenv(key, value)
	}
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

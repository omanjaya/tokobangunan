package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	AppEnv        string
	Port          string
	DatabaseURL   string
	LogLevel      string
	LogFormat     string
	SessionSecret string
}

func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:        getEnv("APP_ENV", "development"),
		Port:          getEnv("APP_PORT", "8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		LogFormat:     getEnv("LOG_FORMAT", "text"),
		SessionSecret: os.Getenv("SESSION_SECRET"),
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validate: %w", err)
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if strings.TrimSpace(c.DatabaseURL) == "" {
		return errors.New("DATABASE_URL is required")
	}
	if strings.TrimSpace(c.SessionSecret) == "" {
		return errors.New("SESSION_SECRET is required")
	}
	// In prod we want stronger guarantees on secret length.
	if c.AppEnv == "production" && len(c.SessionSecret) < 32 {
		return errors.New("SESSION_SECRET must be at least 32 chars in production")
	}
	switch c.LogFormat {
	case "text", "json":
	default:
		return fmt.Errorf("invalid LOG_FORMAT %q (want text|json)", c.LogFormat)
	}
	return nil
}

func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

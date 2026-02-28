package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

const (
	EnvDevelopment = "DEV"
	EnvProduction  = "PROD"
)

type AppConfig struct {
	KeycloakClientID     string
	KeycloakClientSecret string
	KeycloakRealm        string
	KeycloakURL          string
	AppBaseURL           string
	PostgresURL          string
	SMTPHost             string
	SMTPPort             string
	SMTPFrom             string
	SMTPPassword         string
	PostPollIntervalMs   int
	AppEnv               string // EnvDevelopment or EnvProduction
	LogLevel             slog.Level
	EnableArcticShift    bool
}

var Config AppConfig

func LoadConfig() {
	cfg := AppConfig{}

	cfg.AppEnv = os.Getenv("APP_ENV")
	cfg.KeycloakClientID = loadRequired("KEYCLOAK_CLIENT_ID")
	cfg.KeycloakClientSecret = loadRequired("KEYCLOAK_CLIENT_SECRET")
	cfg.KeycloakRealm = loadRequired("KEYCLOAK_REALM")
	cfg.KeycloakURL = loadRequired("KEYCLOAK_URL")
	cfg.AppBaseURL = loadRequired("APP_BASE_URL")
	cfg.PostgresURL = loadRequired("POSTGRES_URL")
	cfg.SMTPHost = loadRequired("SMTP_HOST")
	cfg.SMTPPort = loadRequired("SMTP_PORT")
	cfg.SMTPFrom = loadRequired("SMTP_FROM")
	cfg.SMTPPassword = loadRequired("SMTP_PASSWORD")
	cfg.PostPollIntervalMs = parseIntEnv(loadOptional("POST_POLL_INTERVAL_MS", "3000"))
	cfg.EnableArcticShift = parseBoolEnv(loadOptional("ENABLE_ARCTICSHIFT_POLLING", "true"))

	lvlString := loadOptional("LOG_LEVEL", "INFO")
	var err error
	cfg.LogLevel, err = parseLogLevel(lvlString)
	if err != nil {
		slog.Error("Invalid LOG_LEVEL", "error", err)
		cfg.LogLevel = slog.LevelInfo
	}

	Config = cfg
}

func parseLogLevel(s string) (slog.Level, error) {
	var level slog.Level
	var err = level.UnmarshalText([]byte(s))
	return level, err
}

func parseIntEnv(str string) int {
	var value int
	_, err := fmt.Sscanf(str, "%d", &value)
	if err != nil {
		slog.Error("Invalid integer env var", "var", str, "error", err)
		os.Exit(1)
	}
	return value
}

func parseBoolEnv(str string) bool {
	lowerStr := strings.ToLower(str)
	if lowerStr == "true" || lowerStr == "1" || lowerStr == "yes" {
		return true
	}
	return false
}

func loadRequired(key string) string {
	value := os.Getenv(key)
	if value == "" {
		slog.Error("Required env var not set", "key", key)
		os.Exit(1)
	}
	return value
}

func loadOptional(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func (c AppConfig) IsProduction() bool {
	return Config.AppEnv == EnvProduction
}

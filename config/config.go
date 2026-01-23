package config

import (
	"log/slog"
	"os"
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
	PostgresURL          string
	AppEnv               string // EnvDevelopment or EnvProduction
	LogLevel             slog.Level
}

var Config AppConfig

func LoadConfig() {
	cfg := AppConfig{}

	cfg.AppEnv = os.Getenv("APP_ENV")
	cfg.KeycloakClientID = loadRequired("KEYCLOAK_CLIENT_ID")
	cfg.KeycloakClientSecret = loadRequired("KEYCLOAK_CLIENT_SECRET")
	cfg.KeycloakRealm = loadRequired("KEYCLOAK_REALM")
	cfg.KeycloakURL = loadRequired("KEYCLOAK_URL")
	cfg.PostgresURL = loadRequired("POSTGRES_URL")

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

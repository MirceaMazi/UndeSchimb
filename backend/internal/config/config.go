package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DatabaseURL    string
	Port           string
	CorsOrigin     string
	MigrationsPath string
	RefreshEvery   time.Duration
	HTTPTimeout    time.Duration
	XTBHolidays    map[string]struct{}
}

func Load() Config {
	return Config{
		DatabaseURL:    env("DATABASE_URL", "postgres://undeschimb:undeschimb@localhost:5432/undeschimb?sslmode=disable"),
		Port:           env("PORT", "8080"),
		CorsOrigin:     env("CORS_ORIGIN", "http://localhost:5173"),
		MigrationsPath: env("MIGRATIONS_PATH", "migrations/001_init.sql"),
		RefreshEvery:   durationEnv("REFRESH_EVERY", 15*time.Minute),
		HTTPTimeout:    durationEnv("HTTP_TIMEOUT", 12*time.Second),
		XTBHolidays:    holidaySet(os.Getenv("XTB_HOLIDAYS")),
	}
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
		if value, err := time.ParseDuration(raw); err == nil && value > 0 {
			return value
		}
		if minutes, err := strconv.Atoi(raw); err == nil && minutes > 0 {
			return time.Duration(minutes) * time.Minute
		}
	}
	return fallback
}

func holidaySet(raw string) map[string]struct{} {
	values := make(map[string]struct{})
	for _, value := range strings.Split(raw, ",") {
		if date := strings.TrimSpace(value); date != "" {
			values[date] = struct{}{}
		}
	}
	return values
}


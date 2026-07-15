// Package config loads deploy-time settings from the environment.
package config

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

const (
	defaultEmptyCleanupSeconds        = 30
	defaultOwnerAbsenceHandoffSeconds = 60
)

// Config holds deploy-time settings read from .env / the process environment.
type Config struct {
	DiscordToken               string
	DiscordAppID               string
	DiscordPublicKey           string
	DBPath                     string
	EmptyCleanupSeconds        int
	OwnerAbsenceHandoffSeconds int
}

// Load reads .env (if present) and the process environment into a Config.
// It hard-fails on missing required variables rather than silently defaulting.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, reading from process environment")
	}

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("DISCORD_TOKEN is not set")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		return nil, fmt.Errorf("DB_PATH is not set")
	}

	emptyCleanup, err := intEnvOrDefault("EMPTY_CLEANUP_SECONDS", defaultEmptyCleanupSeconds)
	if err != nil {
		return nil, err
	}

	ownerAbsence, err := intEnvOrDefault("OWNER_ABSENCE_HANDOFF_SECONDS", defaultOwnerAbsenceHandoffSeconds)
	if err != nil {
		return nil, err
	}

	return &Config{
		DiscordToken:               token,
		DiscordAppID:               os.Getenv("DISCORD_APP_ID"),
		DiscordPublicKey:           os.Getenv("DISCORD_PUBLIC_KEY"),
		DBPath:                     dbPath,
		EmptyCleanupSeconds:        emptyCleanup,
		OwnerAbsenceHandoffSeconds: ownerAbsence,
	}, nil
}

func intEnvOrDefault(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer, got %q", key, v)
	}
	return n, nil
}

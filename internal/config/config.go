// Package config loads deploy-time settings from the environment.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/joho/godotenv"

	"xlparties/internal/logger"
)

const (
	defaultEmptyCleanupSeconds        = 30
	defaultOwnerAbsenceHandoffSeconds = 60

	appDataDirName = "xlparties"
	dbFileName     = "xlparties.db"
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
// DB_PATH is the one exception: if unset, it defaults to the XDG data
// directory (see defaultDBPath) rather than failing, since that location is
// a documented, predictable standard rather than a silent guess.
func Load() (*Config, error) {
	if err := godotenv.Load(); err != nil {
		logger.Info("no .env file found, reading from process environment")
	}

	token := os.Getenv("DISCORD_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("DISCORD_TOKEN is not set")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		var err error
		dbPath, err = defaultDBPath()
		if err != nil {
			return nil, fmt.Errorf("resolve default DB_PATH: %w", err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create DB_PATH directory %q: %w", filepath.Dir(dbPath), err)
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

// defaultDBPath returns $XDG_DATA_HOME/xlparties/xlparties.db, falling back
// to ~/.local/share/xlparties/xlparties.db per the XDG Base Directory spec
// when XDG_DATA_HOME is unset.
func defaultDBPath() (string, error) {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataHome, appDataDirName, dbFileName), nil
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

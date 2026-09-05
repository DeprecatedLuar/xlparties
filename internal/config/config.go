// Package config loads deploy-time settings from the environment.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"

	"xlparties/internal/logger"
)

const (
	defaultEmptyCleanupSeconds        = 30
	defaultOwnerAbsenceHandoffSeconds = 60
	defaultInviteExpirySeconds        = 1800

	appDataDirName = "xlparties"
	dbFileName     = "xlparties.db"

	listSeparator = ","
)

// AlwaysAllowedRolesEnv is the variable holding the comma-separated role
// names. Exported so the error path that resolves those names can name it.
const AlwaysAllowedRolesEnv = "ALWAYS_ALLOWED_ROLES"

// Config holds deploy-time settings read from .env / the process environment.
type Config struct {
	DiscordToken               string
	DiscordAppID               string
	DiscordPublicKey           string
	DBPath                     string
	EmptyCleanupSeconds        int
	OwnerAbsenceHandoffSeconds int
	InviteExpirySeconds        int

	// AlwaysAllowedRoles are guild role names (not ids) that get an allow
	// overwrite on every party channel in every access mode - the way a music
	// bot or a staff role gets into an otherwise private party. Names are
	// resolved to ids against the live guild at startup.
	AlwaysAllowedRoles []string
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

	inviteExpiry, err := intEnvOrDefault("INVITE_EXPIRY_SECONDS", defaultInviteExpirySeconds)
	if err != nil {
		return nil, err
	}

	return &Config{
		AlwaysAllowedRoles:         splitList(os.Getenv(AlwaysAllowedRolesEnv)),
		DiscordToken:               token,
		DiscordAppID:               os.Getenv("DISCORD_APP_ID"),
		DiscordPublicKey:           os.Getenv("DISCORD_PUBLIC_KEY"),
		DBPath:                     dbPath,
		EmptyCleanupSeconds:        emptyCleanup,
		OwnerAbsenceHandoffSeconds: ownerAbsence,
		InviteExpirySeconds:        inviteExpiry,
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

// splitList turns a comma-separated env value into a trimmed list, dropping
// empty entries so trailing separators and spacing are harmless.
func splitList(value string) []string {
	var items []string
	for _, item := range strings.Split(value, listSeparator) {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
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

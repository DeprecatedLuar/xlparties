package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrateSchemaAddsMissingColumn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	// Pre-access_mode parties table, as it would exist on a DB created
	// before that column was added to schema.sql.
	if _, err := db.Exec(`
		CREATE TABLE parties (
			channel_id INTEGER PRIMARY KEY,
			owner_id   INTEGER NOT NULL,
			created_at INTEGER NOT NULL
		)
	`); err != nil {
		t.Fatalf("create old parties table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO parties (channel_id, owner_id, created_at) VALUES (1, 2, 3)`); err != nil {
		t.Fatalf("insert row: %v", err)
	}

	if err := migrateSchema(db); err != nil {
		t.Fatalf("migrateSchema: %v", err)
	}

	var accessMode string
	if err := db.QueryRow(`SELECT access_mode FROM parties WHERE channel_id = 1`).Scan(&accessMode); err != nil {
		t.Fatalf("query access_mode after migration: %v", err)
	}
	if accessMode != AccessModeFriendsOfFriends {
		t.Errorf("access_mode = %q, want %q", accessMode, AccessModeFriendsOfFriends)
	}
}

func TestMigrateSchemaWidensAccessModeCheck(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	// parties table as it would exist after only the access_mode column was
	// added, before invite_only was a valid value.
	if _, err := db.Exec(`
		CREATE TABLE parties (
			channel_id  INTEGER PRIMARY KEY,
			owner_id    INTEGER NOT NULL,
			created_at  INTEGER NOT NULL,
			access_mode TEXT NOT NULL DEFAULT 'friends_of_friends' CHECK (access_mode IN ('friends_of_friends','friends_only'))
		)
	`); err != nil {
		t.Fatalf("create pre-invite_only parties table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO parties (channel_id, owner_id, created_at, access_mode) VALUES (1, 2, 3, 'friends_only')`); err != nil {
		t.Fatalf("insert row: %v", err)
	}

	if err := migrateSchema(db); err != nil {
		t.Fatalf("migrateSchema: %v", err)
	}

	if _, err := db.Exec(`UPDATE parties SET access_mode = 'invite_only' WHERE channel_id = 1`); err != nil {
		t.Fatalf("update to invite_only after migration should be accepted by the widened CHECK: %v", err)
	}

	var accessMode string
	if err := db.QueryRow(`SELECT access_mode FROM parties WHERE channel_id = 1`).Scan(&accessMode); err != nil {
		t.Fatalf("query access_mode after migration: %v", err)
	}
	if accessMode != AccessModeInviteOnly {
		t.Errorf("access_mode = %q, want %q", accessMode, AccessModeInviteOnly)
	}
}

func TestMigrateSchemaIdempotent(t *testing.T) {
	// Open already runs migrateSchema once; a second call against the same
	// already-current DB must not error (e.g. duplicate-column).
	s := openTestStore(t)

	if err := migrateSchema(s.db); err != nil {
		t.Fatalf("second migrateSchema (should be no-op): %v", err)
	}
}

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

func TestMigrateSchemaIdempotent(t *testing.T) {
	// Open already runs migrateSchema once; a second call against the same
	// already-current DB must not error (e.g. duplicate-column).
	s := openTestStore(t)

	if err := migrateSchema(s.db); err != nil {
		t.Fatalf("second migrateSchema (should be no-op): %v", err)
	}
}

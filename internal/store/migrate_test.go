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

func TestMigrateSchemaWidensAccessModeCheckForPublic(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	// parties table as it would exist after invite_only was added, before
	// public was a valid value.
	if _, err := db.Exec(`
		CREATE TABLE parties (
			channel_id  INTEGER PRIMARY KEY,
			owner_id    INTEGER NOT NULL,
			created_at  INTEGER NOT NULL,
			access_mode TEXT NOT NULL DEFAULT 'friends_of_friends' CHECK (access_mode IN ('friends_of_friends','friends_only','invite_only'))
		)
	`); err != nil {
		t.Fatalf("create pre-public parties table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO parties (channel_id, owner_id, created_at, access_mode) VALUES (1, 2, 3, 'invite_only')`); err != nil {
		t.Fatalf("insert row: %v", err)
	}

	if err := migrateSchema(db); err != nil {
		t.Fatalf("migrateSchema: %v", err)
	}

	if _, err := db.Exec(`UPDATE parties SET access_mode = 'public' WHERE channel_id = 1`); err != nil {
		t.Fatalf("update to public after migration should be accepted by the widened CHECK: %v", err)
	}

	var accessMode string
	if err := db.QueryRow(`SELECT access_mode FROM parties WHERE channel_id = 1`).Scan(&accessMode); err != nil {
		t.Fatalf("query access_mode after migration: %v", err)
	}
	if accessMode != AccessModePublic {
		t.Errorf("access_mode = %q, want %q", accessMode, AccessModePublic)
	}
}

func TestMigrateRelationshipsFlags(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	// Current-shape parties table so the unrelated parties migrations in
	// migrateSchema are no-ops for this test.
	if _, err := db.Exec(`
		CREATE TABLE parties (
			channel_id  INTEGER PRIMARY KEY,
			owner_id    INTEGER NOT NULL,
			created_at  INTEGER NOT NULL,
			access_mode TEXT NOT NULL DEFAULT 'public' CHECK (access_mode IN ('friends_of_friends','friends_only','invite_only','public'))
		)
	`); err != nil {
		t.Fatalf("create parties table: %v", err)
	}

	// Old single-slot relationships table, as it would exist before the
	// friend/block flags split.
	if _, err := db.Exec(`
		CREATE TABLE relationships (
			granter_id    INTEGER NOT NULL,
			grantee_id    INTEGER NOT NULL,
			relation_type TEXT NOT NULL CHECK (relation_type IN ('friend','block')),
			created_at    INTEGER NOT NULL,
			PRIMARY KEY (granter_id, grantee_id)
		)
	`); err != nil {
		t.Fatalf("create old relationships table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO relationships (granter_id, grantee_id, relation_type, created_at) VALUES (1, 2, 'friend', 100)`); err != nil {
		t.Fatalf("insert friend row: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO relationships (granter_id, grantee_id, relation_type, created_at) VALUES (1, 3, 'block', 200)`); err != nil {
		t.Fatalf("insert block row: %v", err)
	}

	if err := migrateSchema(db); err != nil {
		t.Fatalf("migrateSchema: %v", err)
	}

	var isFriend, isBlocked int
	if err := db.QueryRow(`SELECT is_friend, is_blocked FROM relationships WHERE granter_id = 1 AND grantee_id = 2`).Scan(&isFriend, &isBlocked); err != nil {
		t.Fatalf("query migrated friend row: %v", err)
	}
	if isFriend != 1 || isBlocked != 0 {
		t.Fatalf("migrated friend row = (is_friend=%d, is_blocked=%d), want (1,0)", isFriend, isBlocked)
	}
	if err := db.QueryRow(`SELECT is_friend, is_blocked FROM relationships WHERE granter_id = 1 AND grantee_id = 3`).Scan(&isFriend, &isBlocked); err != nil {
		t.Fatalf("query migrated block row: %v", err)
	}
	if isFriend != 0 || isBlocked != 1 {
		t.Fatalf("migrated block row = (is_friend=%d, is_blocked=%d), want (0,1)", isFriend, isBlocked)
	}

	var indexCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = 'idx_grantee'`).Scan(&indexCount); err != nil {
		t.Fatalf("check idx_grantee: %v", err)
	}
	if indexCount != 1 {
		t.Fatalf("idx_grantee missing after migration, count = %d", indexCount)
	}
}

func TestMigrateSchemaAddsUserLimitColumn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	// Pre-user_limit user_presets table, as it would exist on a DB created
	// before that column was added to schema.sql.
	if _, err := db.Exec(`
		CREATE TABLE user_presets (
			user_id     INTEGER PRIMARY KEY,
			access_mode TEXT NOT NULL CHECK (access_mode IN ('friends_of_friends','friends_only','invite_only','public'))
		)
	`); err != nil {
		t.Fatalf("create old user_presets table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO user_presets (user_id, access_mode) VALUES (1, 'public')`); err != nil {
		t.Fatalf("insert row: %v", err)
	}

	if err := migrateSchema(db); err != nil {
		t.Fatalf("migrateSchema: %v", err)
	}

	var userLimit int
	if err := db.QueryRow(`SELECT user_limit FROM user_presets WHERE user_id = 1`).Scan(&userLimit); err != nil {
		t.Fatalf("query user_limit after migration: %v", err)
	}
	if userLimit != 0 {
		t.Errorf("user_limit = %d, want 0", userLimit)
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

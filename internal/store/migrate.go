package store

import (
	"database/sql"
	"fmt"
	"strings"

	"xlparties/internal/logger"
)

// column describes one column this schema version expects to exist, for
// tables that may pre-date it. name/ddl must match schema.sql exactly.
type column struct {
	name string
	ddl  string // fragment after "ADD COLUMN <name> "
}

// expectedColumns lists, per table, every column that may be missing from a
// DB created before that column was added to schema.sql. schema.sql remains
// the source of truth for fresh installs; this map only repairs existing
// ones. Only append to it when adding a column to an existing table -
// columns present since a table's original schema.sql version don't need an
// entry here.
var expectedColumns = map[string][]column{
	"parties": {
		{name: "access_mode", ddl: "TEXT NOT NULL DEFAULT 'friends_of_friends' CHECK (access_mode IN ('friends_of_friends','friends_only','invite_only','public'))"},
	},
	"user_presets": {
		{name: "user_limit", ddl: "INTEGER NOT NULL DEFAULT 0 CHECK (user_limit BETWEEN 0 AND 99)"},
	},
}

// migrateSchema adds any column listed in expectedColumns that is missing
// from its table, for DBs created before that column existed, then widens
// constraints that have changed shape since. Safe to call on every startup:
// anything already current is left untouched.
func migrateSchema(db *sql.DB) error {
	for table, columns := range expectedColumns {
		exists, err := tableExists(db, table)
		if err != nil {
			return fmt.Errorf("check table %s exists: %w", table, err)
		}
		if !exists {
			// schema.sql's CREATE TABLE IF NOT EXISTS already runs before
			// migrateSchema in Open() and creates the table in its current
			// (already-columned) shape, so there is nothing to patch here.
			continue
		}
		existing, err := existingColumns(db, table)
		if err != nil {
			return fmt.Errorf("read columns for %s: %w", table, err)
		}
		for _, col := range columns {
			if existing[col.name] {
				continue
			}
			stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, col.name, col.ddl)
			if _, err := db.Exec(stmt); err != nil {
				return fmt.Errorf("add column %s.%s: %w", table, col.name, err)
			}
			logger.Info("store: migrated schema, added column", "table", table, "column", col.name)
		}
	}
	if err := migratePartiesAccessModeCheck(db); err != nil {
		return fmt.Errorf("widen parties.access_mode check: %w", err)
	}
	if err := migrateRelationshipsFlags(db); err != nil {
		return fmt.Errorf("migrate relationships to friend/block flags: %w", err)
	}
	return nil
}

// migrateRelationshipsFlags rebuilds the relationships table if it still
// carries the old single-slot relation_type column, replacing it with
// independent is_friend/is_blocked flags. A no-op once the table already
// matches schema.sql.
func migrateRelationshipsFlags(db *sql.DB) error {
	var tableSQL sql.NullString
	err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'relationships'`).Scan(&tableSQL)
	if err == sql.ErrNoRows {
		return nil // fresh schema.sql apply already created the current shape
	}
	if err != nil {
		return fmt.Errorf("read relationships table definition: %w", err)
	}
	if strings.Contains(tableSQL.String, "is_friend") {
		return nil // already current
	}

	statements := []string{
		"PRAGMA foreign_keys = OFF",
		`CREATE TABLE relationships_new (
			granter_id INTEGER NOT NULL REFERENCES users(id),
			grantee_id INTEGER NOT NULL REFERENCES users(id),
			is_friend  INTEGER NOT NULL DEFAULT 0 CHECK (is_friend IN (0,1)),
			is_blocked INTEGER NOT NULL DEFAULT 0 CHECK (is_blocked IN (0,1)),
			created_at INTEGER NOT NULL,
			PRIMARY KEY (granter_id, grantee_id)
		)`,
		`INSERT INTO relationships_new (granter_id, grantee_id, is_friend, is_blocked, created_at)
			SELECT granter_id, grantee_id, relation_type = 'friend', relation_type = 'block', created_at FROM relationships`,
		`DROP TABLE relationships`,
		`ALTER TABLE relationships_new RENAME TO relationships`,
		`CREATE INDEX IF NOT EXISTS idx_grantee ON relationships(grantee_id)`,
		"PRAGMA foreign_keys = ON",
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	logger.Info("store: migrated schema, split relationships.relation_type into is_friend/is_blocked flags")
	return nil
}

// migratePartiesAccessModeCheck rebuilds the parties table if its
// access_mode CHECK constraint predates the invite_only or public modes.
// SQLite has no ALTER TABLE form for changing a CHECK constraint, so the
// only way to widen one on an existing table is to recreate it under the DDL
// in schema.sql and copy the data across. A no-op once the table already
// matches.
func migratePartiesAccessModeCheck(db *sql.DB) error {
	var tableSQL sql.NullString
	err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'parties'`).Scan(&tableSQL)
	if err == sql.ErrNoRows {
		return nil // fresh schema.sql apply already created the current shape
	}
	if err != nil {
		return fmt.Errorf("read parties table definition: %w", err)
	}
	if strings.Contains(tableSQL.String, "public") {
		return nil // already current
	}

	statements := []string{
		"PRAGMA foreign_keys = OFF",
		`CREATE TABLE parties_new (
			channel_id  INTEGER PRIMARY KEY,
			owner_id    INTEGER NOT NULL,
			created_at  INTEGER NOT NULL,
			access_mode TEXT NOT NULL DEFAULT 'friends_of_friends' CHECK (access_mode IN ('friends_of_friends','friends_only','invite_only','public'))
		)`,
		`INSERT INTO parties_new (channel_id, owner_id, created_at, access_mode) SELECT channel_id, owner_id, created_at, access_mode FROM parties`,
		`DROP TABLE parties`,
		`ALTER TABLE parties_new RENAME TO parties`,
		"PRAGMA foreign_keys = ON",
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("exec %q: %w", stmt, err)
		}
	}
	logger.Info("store: migrated schema, widened parties.access_mode check to include public")
	return nil
}

// tableExists reports whether table exists in db.
func tableExists(db *sql.DB, table string) (bool, error) {
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// existingColumns returns the set of column names currently on table.
func existingColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dfltValue, &pk); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

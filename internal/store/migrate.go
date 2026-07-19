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
}

// migrateSchema adds any column listed in expectedColumns that is missing
// from its table, for DBs created before that column existed, then widens
// constraints that have changed shape since. Safe to call on every startup:
// anything already current is left untouched.
func migrateSchema(db *sql.DB) error {
	for table, columns := range expectedColumns {
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

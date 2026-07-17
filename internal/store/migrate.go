package store

import (
	"database/sql"
	"fmt"
	"log"
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
		{name: "access_mode", ddl: "TEXT NOT NULL DEFAULT 'friends_of_friends' CHECK (access_mode IN ('friends_of_friends','friends_only'))"},
	},
}

// migrateSchema adds any column listed in expectedColumns that is missing
// from its table, for DBs created before that column existed. Safe to call
// on every startup: columns already present are left untouched.
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
			log.Printf("store: migrated schema - added column %s.%s", table, col.name)
		}
	}
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

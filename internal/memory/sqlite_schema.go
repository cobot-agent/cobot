package memory

import (
	"database/sql"
	_ "embed"
	"fmt"
)

const schemaVersion = 1

//go:embed embed_schema.sql
var schemaSQL string

// ensureSchema creates all tables, indexes, and FTS5 virtual tables if they
// don't exist, then stamps the schema version.
func ensureSchema(db *sql.DB) error {
	if _, err := db.Exec(schemaSQL); err != nil {
		return err
	}
	_, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion))
	return err
}

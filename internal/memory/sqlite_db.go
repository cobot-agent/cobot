package memory

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// openDB opens a SQLite database at the given directory with WAL mode,
// foreign keys enabled, and a busy timeout for concurrent access.
func openDB(memoryDir string) (*sql.DB, error) {
	if err := os.MkdirAll(memoryDir, 0755); err != nil {
		return nil, fmt.Errorf("create memory dir: %w", err)
	}

	dbPath := filepath.Join(memoryDir, "memory.db")
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)", dbPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Single writer connection to avoid SQLITE_BUSY on writes.
	// WAL mode allows concurrent readers regardless.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	return db, nil
}

func newID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b)
}

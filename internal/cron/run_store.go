package cron

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// RunRecord represents a single cron job execution result.
type RunRecord struct {
	ID       string    `json:"id"`
	JobID    string    `json:"job_id"`
	RunAt    time.Time `json:"run_at"`
	Duration int64     `json:"duration_ms"`
	Result   string    `json:"result"`
	Error    string    `json:"error,omitempty"`
}

// RunStore manages per-job SQLite databases for execution records.
type RunStore struct {
	dir string   // base directory: <workspace>/cron/result/
	dbs sync.Map // jobID -> *sql.DB
}

// NewRunStore creates a RunStore backed by the given directory.
func NewRunStore(dir string) *RunStore {
	return &RunStore{dir: dir}
}

func (rs *RunStore) dbPath(jobID string) string {
	return filepath.Join(rs.dir, jobID+".db")
}

func (rs *RunStore) ensureSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS runs (
		id TEXT PRIMARY KEY,
		run_at TEXT NOT NULL,
		duration_ms INTEGER NOT NULL,
		result TEXT NOT NULL DEFAULT '',
		error TEXT NOT NULL DEFAULT ''
	)`)
	if err != nil {
		return fmt.Errorf("create runs table: %w", err)
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_runs_run_at ON runs(run_at)`)
	return err
}

// getDB returns a cached *sql.DB for the given jobID, opening and caching one
// if necessary. Concurrent calls are safe; only the first opener's DB is kept.
func (rs *RunStore) getDB(jobID string) (*sql.DB, error) {
	if v, ok := rs.dbs.Load(jobID); ok {
		return v.(*sql.DB), nil
	}
	if err := os.MkdirAll(rs.dir, 0755); err != nil {
		return nil, fmt.Errorf("create run store dir: %w", err)
	}
	db, err := sql.Open("sqlite", rs.dbPath(jobID))
	if err != nil {
		return nil, fmt.Errorf("open run db for job %s: %w", jobID, err)
	}
	if err := rs.ensureSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	// Store-or-load to handle concurrent access.
	if actual, loaded := rs.dbs.LoadOrStore(jobID, db); loaded {
		db.Close() // another goroutine won the race
		return actual.(*sql.DB), nil
	}
	return db, nil
}

// Close closes all cached database connections.
func (rs *RunStore) Close() {
	rs.dbs.Range(func(key, value any) bool {
		value.(*sql.DB).Close()
		return true
	})
}

func (rs *RunStore) withDB(jobID string, fn func(*sql.DB) error) error {
	if err := ValidateJobID(jobID); err != nil {
		return err
	}
	db, err := rs.getDB(jobID)
	if err != nil {
		return err
	}
	return fn(db)
}

// StoreRun saves a single execution record.
func (rs *RunStore) StoreRun(record *RunRecord) error {
	return rs.withDB(record.JobID, func(db *sql.DB) error {
		_, err := db.Exec(`INSERT INTO runs (id, run_at, duration_ms, result, error) VALUES (?, ?, ?, ?, ?)`,
			record.ID, record.RunAt.Format(time.RFC3339), record.Duration, record.Result, record.Error)
		return err
	})
}

// ListRuns returns the most recent runs for a job, limited by limit.
func (rs *RunStore) ListRuns(jobID string, limit int) ([]*RunRecord, error) {
	if limit <= 0 {
		limit = 20
	}
	if err := ValidateJobID(jobID); err != nil {
		return nil, err
	}
	// Don't create a DB file just to list runs.
	if _, err := os.Stat(rs.dbPath(jobID)); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	db, err := rs.getDB(jobID)
	if err != nil {
		return nil, err
	}
	var records []*RunRecord
	rows, err := db.Query(`SELECT id, run_at, duration_ms, result, error FROM runs ORDER BY run_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		r := &RunRecord{JobID: jobID}
		var runAt string
		if err := rows.Scan(&r.ID, &runAt, &r.Duration, &r.Result, &r.Error); err != nil {
			return nil, err
		}
		r.RunAt, err = time.Parse(time.RFC3339, runAt)
		if err != nil {
			return nil, fmt.Errorf("parse run_at timestamp for job %s: %w", jobID, err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// DeleteJobDB removes the entire database for a job.
// Returns nil if the database does not exist (idempotent).
// Also removes SQLite WAL and SHM sidecar files.
func (rs *RunStore) DeleteJobDB(jobID string) error {
	if err := ValidateJobID(jobID); err != nil {
		return err
	}
	// Close and remove from cache before deleting files.
	if v, ok := rs.dbs.LoadAndDelete(jobID); ok {
		v.(*sql.DB).Close()
	}
	for _, suffix := range []string{"", "-wal", "-shm"} {
		if err := os.Remove(rs.dbPath(jobID) + suffix); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

// RunsExist checks if any run records exist for a job.
func (rs *RunStore) RunsExist(jobID string) (bool, error) {
	if err := ValidateJobID(jobID); err != nil {
		return false, err
	}
	dbPath := rs.dbPath(jobID)
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	var count int
	err := rs.withDB(jobID, func(db *sql.DB) error {
		return db.QueryRow(`SELECT COUNT(*) FROM runs`).Scan(&count)
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

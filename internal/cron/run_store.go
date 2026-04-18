package cron

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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
	dir string // base directory: <workspace>/cron_runs/
}

// NewRunStore creates a RunStore backed by the given directory.
func NewRunStore(dir string) *RunStore {
	return &RunStore{dir: dir}
}

func (rs *RunStore) dbPath(jobID string) string {
	return filepath.Join(rs.dir, jobID+".db")
}

func (rs *RunStore) openDB(jobID string) (*sql.DB, error) {
	if err := os.MkdirAll(rs.dir, 0755); err != nil {
		return nil, fmt.Errorf("create cron_runs dir: %w", err)
	}
	db, err := sql.Open("sqlite", rs.dbPath(jobID))
	if err != nil {
		return nil, fmt.Errorf("open run db for job %s: %w", jobID, err)
	}
	return db, nil
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

// StoreRun saves a single execution record.
func (rs *RunStore) StoreRun(record *RunRecord) error {
	db, err := rs.openDB(record.JobID)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := rs.ensureSchema(db); err != nil {
		return err
	}
	_, err = db.Exec(`INSERT INTO runs (id, run_at, duration_ms, result, error) VALUES (?, ?, ?, ?, ?)`,
		record.ID, record.RunAt.Format(time.RFC3339), record.Duration, record.Result, record.Error)
	return err
}

// ListRuns returns the most recent runs for a job, limited by limit.
func (rs *RunStore) ListRuns(jobID string, limit int) ([]*RunRecord, error) {
	db, err := rs.openDB(jobID)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := rs.ensureSchema(db); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.Query(`SELECT id, run_at, duration_ms, result, error FROM runs ORDER BY run_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []*RunRecord
	for rows.Next() {
		r := &RunRecord{JobID: jobID}
		var runAt string
		if err := rows.Scan(&r.ID, &runAt, &r.Duration, &r.Result, &r.Error); err != nil {
			return nil, err
		}
		r.RunAt, _ = time.Parse(time.RFC3339, runAt)
		records = append(records, r)
	}
	return records, rows.Err()
}

// DeleteJobDB removes the entire database for a job.
func (rs *RunStore) DeleteJobDB(jobID string) error {
	return os.Remove(rs.dbPath(jobID))
}

// RunsExist checks if any run records exist for a job.
func (rs *RunStore) RunsExist(jobID string) (bool, error) {
	path := rs.dbPath(jobID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	}
	db, err := rs.openDB(jobID)
	if err != nil {
		return false, err
	}
	defer db.Close()
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM runs`).Scan(&count)
	return count > 0, err
}

// ConsolidateJobRuns returns all run results as a summary string for LTM storage.
func (rs *RunStore) ConsolidateJobRuns(jobID, jobName string) (string, error) {
	records, err := rs.ListRuns(jobID, 1000)
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		return "", nil
	}
	summary := fmt.Sprintf("Cron job %q (id=%s) execution history (%d runs):\n", jobName, jobID, len(records))
	for i := len(records) - 1; i >= 0; i-- {
		r := records[i]
		if r.Error != "" {
			summary += fmt.Sprintf("- [%s] FAILED: %s\n", r.RunAt.Format("2006-01-02 15:04"), r.Error)
		} else {
			result := r.Result
			if len(result) > 200 {
				result = result[:200] + "..."
			}
			summary += fmt.Sprintf("- [%s] %s\n", r.RunAt.Format("2006-01-02 15:04"), result)
		}
	}
	return summary, nil
}

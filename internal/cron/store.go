package cron

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

var validJobID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// ValidateJobID returns an error if the ID contains characters that could
// cause path traversal or is empty.
func ValidateJobID(id string) error {
	if id == "" {
		return fmt.Errorf("job id is empty")
	}
	if !validJobID.MatchString(id) {
		return fmt.Errorf("job id %q contains invalid characters (only alphanumeric, underscore, hyphen allowed)", id)
	}
	return nil
}

// Job represents a scheduled or one-shot cron job.
type Job struct {
	ID        string     `yaml:"id"`
	Name      string     `yaml:"name"`
	Schedule  string     `yaml:"schedule"`
	Prompt    string     `yaml:"prompt"`
	Model     string     `yaml:"model,omitempty"`
	Status    string     `yaml:"status"`
	OneShot   bool       `yaml:"one_shot"`
	CreatedAt time.Time  `yaml:"created_at"`
	LastRun   *time.Time `yaml:"last_run,omitempty"`
	NextRun   *time.Time `yaml:"next_run,omitempty"`
	RunCount  int        `yaml:"run_count"`

	// ReadToken is a random opaque token persisted in YAML, regenerated on
	// every read (list/get) and every mutation (create/update). It guarantees
	// the caller has seen the latest state before performing destructive ops.
	ReadToken string `yaml:"read_token"`

	// Notification target
	ChannelID string `yaml:"channel_id,omitempty"`
}

// ReadID returns a temporary opaque token combining the job ID with the
// current ReadToken. This value must be passed back for delete/pause/resume
// operations. Note: calling ReadID() does NOT regenerate the token — that
// only happens when reading from or writing to the Store.
func (j *Job) ReadID() string {
	return j.ID + ":" + j.ReadToken
}

// generateToken creates a random opaque token. Uses UUID which is already a
// project dependency and provides 122 bits of randomness.
func generateToken() string {
	return uuid.NewString()
}

// ParseReadID splits a read_id token ("<jobID>:<token>") into its components.
// Returns the job ID, expected token, and any parse error.
func ParseReadID(readID string) (jobID string, token string, err error) {
	parts := strings.SplitN(readID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid read_id format: %q (expected \"<job_id>:<token>\")", readID)
	}
	if err := ValidateJobID(parts[0]); err != nil {
		return "", "", err
	}
	return parts[0], parts[1], nil
}

// Store manages Job persistence as individual YAML files.
type Store struct {
	dir string
}

// NewStore creates a new Store backed by the given directory.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) jobPath(id string) string {
	return filepath.Join(s.dir, id+".yaml")
}

// writeJob marshals and writes the job file.
func (s *Store) writeJob(job *Job) error {
	data, err := yaml.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	if err := os.WriteFile(s.jobPath(job.ID), data, 0644); err != nil {
		return fmt.Errorf("write job file: %w", err)
	}
	return nil
}

// Create writes a new job to disk. Generates an initial ReadToken.
func (s *Store) Create(job *Job) error {
	if err := ValidateJobID(job.ID); err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("create cron dir: %w", err)
	}
	job.ReadToken = generateToken()
	return s.writeJob(job)
}

// Get reads a single job by ID, generates a fresh ReadToken, and writes the
// updated token back to disk. This write-on-read behaviour is intentional:
// it implements optimistic concurrency by ensuring every read invalidates any
// previously issued read_id. Callers that need job data without refreshing the
// token (e.g. internal reconciliation) should use Read() instead to avoid the
// unnecessary I/O.
func (s *Store) Get(id string) (*Job, error) {
	if err := ValidateJobID(id); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(s.jobPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("job not found: %s", id)
		}
		return nil, fmt.Errorf("read job file: %w", err)
	}
	var job Job
	if err := yaml.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}
	// Regenerate token on every read.
	job.ReadToken = generateToken()
	if writeErr := s.writeJob(&job); writeErr != nil {
		return nil, fmt.Errorf("refresh read token: %w", writeErr)
	}
	return &job, nil
}

// Read reads a single job by ID without refreshing the ReadToken.
// Used internally by mutations that need the current state but should not
// change the token (the subsequent Update/Delete will handle that).
// If expectedToken is non-empty, validates the token matches before returning.
func (s *Store) Read(id string, expectedToken ...string) (*Job, error) {
	if err := ValidateJobID(id); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(s.jobPath(id))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("job not found: %s", id)
		}
		return nil, fmt.Errorf("read job file: %w", err)
	}
	var job Job
	if err := yaml.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}
	if len(expectedToken) > 0 && expectedToken[0] != "" && job.ReadToken != expectedToken[0] {
		return nil, fmt.Errorf("job %s has been modified since last read. Re-read the job and retry", id)
	}
	return &job, nil
}

// List reads all jobs from the store directory.
// Each job gets a fresh ReadToken (via Get), so every list call invalidates
// all previously issued read_ids.
func (s *Store) List() ([]*Job, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read cron dir: %w", err)
	}
	var jobs []*Job
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		id := entry.Name()[:len(entry.Name())-len(".yaml")]
		job, err := s.Get(id) // Get refreshes the token
		if err != nil {
			continue // skip malformed files
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// ListReadOnly reads all jobs without refreshing ReadTokens.
// Use this for internal sync/reconcile operations that don't need
// optimistic-concurrency tokens, avoiding O(n) file writes.
func (s *Store) ListReadOnly() ([]*Job, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read cron dir: %w", err)
	}
	var jobs []*Job
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		id := entry.Name()[:len(entry.Name())-len(".yaml")]
		data, err := os.ReadFile(s.jobPath(id))
		if err != nil {
			continue
		}
		var job Job
		if err := yaml.Unmarshal(data, &job); err != nil {
			continue
		}
		jobs = append(jobs, &job)
	}
	return jobs, nil
}

// Update rewrites an existing job file. Regenerates ReadToken on each update.
func (s *Store) Update(job *Job) error {
	if err := ValidateJobID(job.ID); err != nil {
		return err
	}
	path := s.jobPath(job.ID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("job not found: %s", job.ID)
	}
	job.ReadToken = generateToken()
	return s.writeJob(job)
}

// Delete removes a job file from disk after verifying the expected ReadToken.
func (s *Store) Delete(id string, expectedToken string) error {
	// NOTE: There is a TOCTOU (time-of-check-time-of-use) race between reading
	// the file to verify the token and the actual os.Remove. An external process
	// could modify the file in that window, causing us to delete a newer version.
	// This is acceptable for the current single-process deployment model. If
	// multi-process access is needed, consider file-locking or a database backend.
	if err := ValidateJobID(id); err != nil {
		return err
	}
	if expectedToken != "" {
		data, err := os.ReadFile(s.jobPath(id))
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("job not found: %s", id)
			}
			return fmt.Errorf("read job file: %w", err)
		}
		var job Job
		if err := yaml.Unmarshal(data, &job); err != nil {
			return fmt.Errorf("unmarshal job: %w", err)
		}
		if job.ReadToken != expectedToken {
			return fmt.Errorf("job %s has been modified since last read. Re-read the job and retry", id)
		}
	}
	if err := os.Remove(s.jobPath(id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete job file: %w", err)
	}
	return nil
}

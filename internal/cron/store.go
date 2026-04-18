package cron

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

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

	// Notification target
	ChannelID string `yaml:"channel_id,omitempty"`
	SessionID string `yaml:"session_id,omitempty"`
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

// Create writes a new job to disk.
func (s *Store) Create(job *Job) error {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return fmt.Errorf("create cron dir: %w", err)
	}
	data, err := yaml.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	if err := os.WriteFile(s.jobPath(job.ID), data, 0644); err != nil {
		return fmt.Errorf("write job file: %w", err)
	}
	return nil
}

// Get reads a single job by ID.
func (s *Store) Get(id string) (*Job, error) {
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
	return &job, nil
}

// List reads all jobs from the store directory.
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
		job, err := s.Get(id)
		if err != nil {
			continue // skip malformed files
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// Update rewrites an existing job file.
func (s *Store) Update(job *Job) error {
	data, err := yaml.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	if err := os.WriteFile(s.jobPath(job.ID), data, 0644); err != nil {
		return fmt.Errorf("write job file: %w", err)
	}
	return nil
}

// Delete removes a job file from disk.
func (s *Store) Delete(id string) error {
	if err := os.Remove(s.jobPath(id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete job file: %w", err)
	}
	return nil
}

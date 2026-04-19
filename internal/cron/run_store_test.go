package cron

import (
	"testing"
	"time"
)

func TestRunStore_StoreAndList(t *testing.T) {
	dir := t.TempDir()
	rs := NewRunStore(dir)
	defer rs.Close()
	jobID := "test_job_1"

	// Store 3 runs with different timestamps.
	for i := 0; i < 3; i++ {
		record := &RunRecord{
			ID:       "run_" + string(rune('A'+i)),
			JobID:    jobID,
			RunAt:    time.Date(2026, 1, 1, 10, i, 0, 0, time.UTC),
			Duration: int64((i + 1) * 100),
			Result:   "result " + string(rune('A'+i)),
			Error:    "",
		}
		if err := rs.StoreRun(record); err != nil {
			t.Fatalf("StoreRun(%d): %v", i, err)
		}
	}

	// List with limit 2 — should get the 2 most recent.
	records, err := rs.ListRuns(jobID, 2)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	// Newest first: run_C (10:02), then run_B (10:01).
	if records[0].ID != "run_C" {
		t.Errorf("expected first record ID run_C, got %s", records[0].ID)
	}
	if records[1].ID != "run_B" {
		t.Errorf("expected second record ID run_B, got %s", records[1].ID)
	}
}

func TestRunStore_DeleteJobDB(t *testing.T) {
	dir := t.TempDir()
	rs := NewRunStore(dir)
	defer rs.Close()
	jobID := "test_job_del"

	record := &RunRecord{
		ID:       "run_1",
		JobID:    jobID,
		RunAt:    time.Now(),
		Duration: 100,
		Result:   "done",
	}
	if err := rs.StoreRun(record); err != nil {
		t.Fatalf("StoreRun: %v", err)
	}

	// Verify DB file exists.
	exists, err := rs.RunsExist(jobID)
	if err != nil {
		t.Fatalf("RunsExist before delete: %v", err)
	}
	if !exists {
		t.Fatal("expected runs to exist before delete")
	}

	// Delete the DB.
	if err := rs.DeleteJobDB(jobID); err != nil {
		t.Fatalf("DeleteJobDB: %v", err)
	}

	// Verify DB file is gone.
	exists, err = rs.RunsExist(jobID)
	if err != nil {
		t.Fatalf("RunsExist after delete: %v", err)
	}
	if exists {
		t.Fatal("expected no runs after delete")
	}
}

func TestRunStore_RunsExist(t *testing.T) {
	dir := t.TempDir()
	rs := NewRunStore(dir)
	defer rs.Close()
	jobID := "test_job_exist"

	// No runs initially.
	exists, err := rs.RunsExist(jobID)
	if err != nil {
		t.Fatalf("RunsExist before: %v", err)
	}
	if exists {
		t.Fatal("expected no runs initially")
	}

	// Store a run.
	record := &RunRecord{
		ID:       "run_1",
		JobID:    jobID,
		RunAt:    time.Now(),
		Duration: 50,
		Result:   "ok",
	}
	if err := rs.StoreRun(record); err != nil {
		t.Fatalf("StoreRun: %v", err)
	}

	// Now runs should exist.
	exists, err = rs.RunsExist(jobID)
	if err != nil {
		t.Fatalf("RunsExist after: %v", err)
	}
	if !exists {
		t.Fatal("expected runs to exist after storing")
	}
}

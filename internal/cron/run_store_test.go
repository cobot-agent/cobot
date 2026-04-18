package cron

import (
	"strings"
	"testing"
	"time"
)

func TestRunStore_StoreAndList(t *testing.T) {
	dir := t.TempDir()
	rs := NewRunStore(dir)
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

func TestRunStore_ConsolidateJobRuns(t *testing.T) {
	dir := t.TempDir()
	rs := NewRunStore(dir)
	jobID := "test_job_consol"

	// Store a success run.
	successRecord := &RunRecord{
		ID:       "run_ok",
		JobID:    jobID,
		RunAt:    time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		Duration: 200,
		Result:   "task completed successfully",
		Error:    "",
	}
	if err := rs.StoreRun(successRecord); err != nil {
		t.Fatalf("StoreRun success: %v", err)
	}

	// Store an error run.
	errorRecord := &RunRecord{
		ID:       "run_err",
		JobID:    jobID,
		RunAt:    time.Date(2026, 4, 1, 13, 0, 0, 0, time.UTC),
		Duration: 50,
		Result:   "",
		Error:    "connection refused",
	}
	if err := rs.StoreRun(errorRecord); err != nil {
		t.Fatalf("StoreRun error: %v", err)
	}

	summary, err := rs.ConsolidateJobRuns(jobID, "daily-check")
	if err != nil {
		t.Fatalf("ConsolidateJobRuns: %v", err)
	}
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}

	// Should contain job name and ID.
	if !contains(summary, "daily-check") {
		t.Error("summary should contain job name")
	}
	if !contains(summary, jobID) {
		t.Error("summary should contain job ID")
	}
	// Should contain FAILED for error run.
	if !contains(summary, "FAILED") {
		t.Error("summary should contain FAILED for error run")
	}
	if !contains(summary, "connection refused") {
		t.Error("summary should contain error message")
	}
	// Should contain success result.
	if !contains(summary, "task completed successfully") {
		t.Error("summary should contain success result")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

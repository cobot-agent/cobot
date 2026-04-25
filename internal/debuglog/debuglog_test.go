package debuglog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func resetGlobal(t *testing.T) {
	t.Helper()
	Close()
}

func TestRotateDeletesOldLogs(t *testing.T) {
	dir := t.TempDir()

	oldFile := filepath.Join(dir, "cobot-2020-01-01.log")
	os.WriteFile(oldFile, []byte("old"), 0o644)
	oldTime := time.Now().AddDate(0, 0, -10)
	os.Chtimes(oldFile, oldTime, oldTime)

	recentFile := filepath.Join(dir, "session-abc-2026-04-17.log")
	os.WriteFile(recentFile, []byte("recent"), 0o644)

	notLog := filepath.Join(dir, "keep.txt")
	os.WriteFile(notLog, []byte("keep"), 0o644)
	os.Chtimes(notLog, oldTime, oldTime)

	rotate(dir, 7)

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatal("old log file should have been deleted")
	}
	if _, err := os.Stat(recentFile); err != nil {
		t.Fatal("recent log file should be kept")
	}
	if _, err := os.Stat(notLog); err != nil {
		t.Fatal("non-.log file should be kept")
	}
}

func TestWithSessionIDAndSessionID(t *testing.T) {
	ctx := context.Background()
	if got := SessionID(ctx); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
	ctx = WithSessionID(ctx, "abc123")
	if got := SessionID(ctx); got != "abc123" {
		t.Fatalf("expected abc123, got %q", got)
	}
}

func TestSessionIDNilContext(t *testing.T) {
	ctx := context.Background()
	if got := SessionID(ctx); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestInitCreatesDir(t *testing.T) {
	defer resetGlobal(t)
	dir := filepath.Join(t.TempDir(), "logs")
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected directory")
	}
}

func TestInitCreatesGlobalLogFile(t *testing.T) {
	defer resetGlobal(t)
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	found := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "cobot-") && strings.HasSuffix(e.Name(), ".log") {
			found = true
		}
	}
	if !found {
		t.Fatal("global log file not created")
	}
}

func TestInitIdempotent(t *testing.T) {
	defer resetGlobal(t)
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	dir2 := t.TempDir()
	if err := Init(dir2); err != nil {
		t.Fatal(err)
	}
	if !Enabled() {
		t.Fatal("expected enabled")
	}
}

func TestEnabledBeforeInit(t *testing.T) {
	defer resetGlobal(t)
	if Enabled() {
		t.Fatal("expected disabled before Init")
	}
}

func TestEnabledAfterInit(t *testing.T) {
	defer resetGlobal(t)
	dir := t.TempDir()
	Init(dir)
	if !Enabled() {
		t.Fatal("expected enabled after Init")
	}
}

func TestEnabledAfterClose(t *testing.T) {
	defer resetGlobal(t)
	dir := t.TempDir()
	Init(dir)
	Close()
	if Enabled() {
		t.Fatal("expected disabled after Close")
	}
}

func TestLogRequestCreatesSessionFile(t *testing.T) {
	defer resetGlobal(t)
	dir := t.TempDir()
	Init(dir)

	ctx := WithSessionID(context.Background(), "sess-abcdefgh-1234")
	LogRequest(ctx, "openai", "https://api.openai.com/v1/chat", []byte(`{"model":"gpt-4"}`))

	entries, _ := os.ReadDir(dir)
	found := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "session-sess-abc") {
			found = true
		}
	}
	if !found {
		t.Fatal("session log file not created")
	}
}

func TestLogRequestNoopWithoutInit(t *testing.T) {
	defer resetGlobal(t)
	ctx := WithSessionID(context.Background(), "sess1")
	LogRequest(ctx, "openai", "https://api.openai.com", []byte(`{}`))
}

func TestLogRequestNoopEmptySession(t *testing.T) {
	defer resetGlobal(t)
	dir := t.TempDir()
	Init(dir)
	LogRequest(context.Background(), "openai", "https://api.openai.com", []byte(`{}`))

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "session-") {
			t.Fatal("should not create session log for empty session ID")
		}
	}
}

func TestLogResponseWritesToFile(t *testing.T) {
	defer resetGlobal(t)
	dir := t.TempDir()
	Init(dir)

	ctx := WithSessionID(context.Background(), "resp-test-session")
	LogResponse(ctx, "anthropic", 200, []byte(`{"content":"hi"}`), 150*time.Millisecond)

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "session-resp-tes") {
			data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
			if !strings.Contains(string(data), "llm response") {
				t.Fatal("expected 'llm response' in log")
			}
			if !strings.Contains(string(data), "anthropic") {
				t.Fatal("expected 'anthropic' in log")
			}
			return
		}
	}
	t.Fatal("session log file not found")
}

func TestLogSSENoSessionCtx(t *testing.T) {
	defer resetGlobal(t)
	// Context without a session ID — getOrCreateLogger returns nil, should not panic.
	LogSSE(context.Background(), "openai", []byte(`data: {"done":true}`))
}

func TestLogSSEWritesToFile(t *testing.T) {
	defer resetGlobal(t)
	dir := t.TempDir()
	Init(dir)

	ctx := WithSessionID(context.Background(), "sse-test-session")
	LogSSE(ctx, "openai", []byte(`{"choices":[{"delta":{}}]}`))

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "session-sse-test") {
			data, _ := os.ReadFile(filepath.Join(dir, e.Name()))
			if !strings.Contains(string(data), "sse event") {
				t.Fatal("expected 'sse event' in log")
			}
			return
		}
	}
	t.Fatal("session log file not found")
}

func TestCloseMultipleTimes(t *testing.T) {
	defer resetGlobal(t)
	dir := t.TempDir()
	Init(dir)
	Close()
	Close()
}

func TestGetOrCreateLoggerReusesLogger(t *testing.T) {
	defer resetGlobal(t)
	dir := t.TempDir()
	Init(dir)

	l1 := getOrCreateLogger("same-session")
	l2 := getOrCreateLogger("same-session")
	if l1 == nil || l2 == nil {
		t.Fatal("expected non-nil loggers")
	}
	if l1 != l2 {
		t.Fatal("expected same logger instance for same session")
	}
}

func TestMultipleSessionsCreateSeparateFiles(t *testing.T) {
	defer resetGlobal(t)
	dir := t.TempDir()
	Init(dir)

	ctx1 := WithSessionID(context.Background(), "aaaa1111-long-id")
	ctx2 := WithSessionID(context.Background(), "bbbb2222-long-id")
	LogRequest(ctx1, "openai", "url1", []byte(`{}`))
	LogRequest(ctx2, "anthropic", "url2", []byte(`{}`))

	entries, _ := os.ReadDir(dir)
	sessionFiles := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "session-") {
			sessionFiles++
		}
	}
	if sessionFiles != 2 {
		t.Fatalf("expected 2 session files, got %d", sessionFiles)
	}
}

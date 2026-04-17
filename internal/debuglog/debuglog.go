// Package debuglog provides session-scoped debug logging for LLM requests,
// responses, and SSE stream events. Logs are written to per-session files
// under ~/.local/share/cobot/logs/ (or $COBOT_DATA_PATH/logs/).
//
// All functions are no-ops when debug logging has not been initialized,
// making it safe to call unconditionally.
package debuglog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ctxKey is an unexported type to avoid collisions with other packages.
type ctxKey struct{}

// sessionIDKey stores the session ID in a context.
var sessionIDKey = ctxKey{}

// WithSessionID returns a copy of ctx carrying the given session ID.
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDKey, id)
}

// SessionID extracts the session ID from ctx, or "" if absent.
func SessionID(ctx context.Context) string {
	if v, ok := ctx.Value(sessionIDKey).(string); ok {
		return v
	}
	return ""
}

// global state guarded by mu.
var (
	mu      sync.RWMutex
	logsDir string
	loggers map[string]*sessionLogger // keyed by sessionID
	global  *os.File                  // global log file (non-session)
)

type sessionLogger struct {
	file   *os.File
	logger *slog.Logger
}

// Init sets up the debug log directory and the global (non-session) log file.
// It must be called once at startup when debug mode is enabled. Subsequent
// calls are no-ops.
func Init(dir string) error {
	mu.Lock()
	defer mu.Unlock()
	if logsDir != "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("debuglog: create logs dir: %w", err)
	}
	logsDir = dir
	loggers = make(map[string]*sessionLogger)

	name := fmt.Sprintf("cobot-%s.log", time.Now().Format("2006-01-02"))
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("debuglog: open global log: %w", err)
	}
	global = f

	w := io.MultiWriter(os.Stderr, f)
	slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})))
	slog.Debug("debug logging initialized", "logs_dir", dir)

	go rotate(dir, 7)
	return nil
}

func rotate(dir string, maxDays int) {
	cutoff := time.Now().AddDate(0, 0, -maxDays)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".log" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			p := filepath.Join(dir, e.Name())
			if err := os.Remove(p); err == nil {
				slog.Debug("rotated old log", "file", e.Name())
			}
		}
	}
}

// Enabled reports whether debug logging has been initialized.
func Enabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return logsDir != ""
}

// getOrCreateLogger returns the session-scoped logger, creating the log file
// on first access. Returns nil if debug logging is not initialized or
// sessionID is empty.
func getOrCreateLogger(sessionID string) *slog.Logger {
	if sessionID == "" {
		return nil
	}
	mu.RLock()
	if logsDir == "" {
		mu.RUnlock()
		return nil
	}
	if sl, ok := loggers[sessionID]; ok {
		mu.RUnlock()
		return sl.logger
	}
	mu.RUnlock()

	mu.Lock()
	defer mu.Unlock()
	if logsDir == "" {
		return nil
	}
	if sl, ok := loggers[sessionID]; ok {
		return sl.logger
	}

	short := sessionID
	if len(short) > 8 {
		short = short[:8]
	}
	name := fmt.Sprintf("session-%s-%s.log", short, time.Now().Format("2006-01-02"))
	f, err := os.OpenFile(filepath.Join(logsDir, name), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		slog.Warn("debuglog: failed to create session log", "session_id", sessionID, "err", err)
		return nil
	}
	l := slog.New(slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug}))
	loggers[sessionID] = &sessionLogger{file: f, logger: l}
	return l
}

// LogRequest logs the full LLM request body (JSON) to the session log.
func LogRequest(ctx context.Context, provider, url string, body []byte) {
	l := getOrCreateLogger(SessionID(ctx))
	if l == nil {
		return
	}
	l.Debug("llm request",
		"provider", provider,
		"url", url,
		"body", string(body),
	)
}

// LogResponse logs a non-streaming LLM response body to the session log.
func LogResponse(ctx context.Context, provider string, status int, body []byte, elapsed time.Duration) {
	l := getOrCreateLogger(SessionID(ctx))
	if l == nil {
		return
	}
	l.Debug("llm response",
		"provider", provider,
		"status", status,
		"elapsed", elapsed.Round(time.Millisecond),
		"body", string(body),
	)
}

// LogSSE logs a single SSE event line to the session log.
func LogSSE(ctx context.Context, provider string, data []byte) {
	if ctx == nil {
		return
	}
	l := getOrCreateLogger(SessionID(ctx))
	if l == nil {
		return
	}
	l.Debug("sse event",
		"provider", provider,
		"data", string(data),
	)
}

// Close flushes and closes all open log files. Safe to call multiple times.
func Close() {
	mu.Lock()
	defer mu.Unlock()
	for _, sl := range loggers {
		sl.file.Close()
	}
	loggers = nil
	if global != nil {
		global.Close()
		global = nil
	}
	logsDir = ""
}

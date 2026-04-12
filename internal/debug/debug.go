package debug

import (
	"io"
	"log/slog"
	"os"
	"sync"
	"time"
)

var (
	mu      sync.RWMutex
	enabled bool
	handler slog.Handler
	logger  *slog.Logger
)

func init() {
	handler = slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger = slog.New(handler)
}

func Enable() {
	mu.Lock()
	defer mu.Unlock()
	enabled = true
	handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger = slog.New(handler)
}

func EnableTo(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	enabled = true
	handler = slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger = slog.New(handler)
}

func Disable() {
	mu.Lock()
	defer mu.Unlock()
	enabled = false
	handler = slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger = slog.New(handler)
}

func IsEnabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return enabled
}

func Logger() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return logger
}

func Log(category, msg string, args ...any) {
	if !IsEnabled() {
		return
	}
	Logger().With("category", category).Info(msg, args...)
}

func Request(provider, method, url string, bodySize int) {
	if !IsEnabled() {
		return
	}
	Logger().With("category", "request", "provider", provider).
		Info("http request", "method", method, "url", url, "body_bytes", bodySize)
}

func Response(provider string, status int, bodySize int, elapsed time.Duration) {
	if !IsEnabled() {
		return
	}
	Logger().With("category", "response", "provider", provider).
		Info("http response", "status", status, "body_bytes", bodySize, "elapsed", elapsed.Round(time.Millisecond).String())
}

func Tool(name string, argsSize int) {
	if !IsEnabled() {
		return
	}
	Logger().With("category", "tool", "name", name).
		Info("executing", "args_bytes", argsSize)
}

func ToolResult(name string, resultSize int, elapsed time.Duration) {
	if !IsEnabled() {
		return
	}
	Logger().With("category", "tool", "name", name).
		Info("completed", "result_bytes", resultSize, "elapsed", elapsed.Round(time.Millisecond).String())
}

func Memory(op, details string) {
	if !IsEnabled() {
		return
	}
	Logger().With("category", "memory").Info(op, "details", details)
}

func Agent(turn int, event, details string) {
	if !IsEnabled() {
		return
	}
	Logger().With("category", "agent").Info(event, "turn", turn, "details", details)
}

func Config(key, value string) {
	if !IsEnabled() {
		return
	}
	Logger().With("category", "config").Info("set", "key", key, "value", value)
}

func Session(event, details string) {
	if !IsEnabled() {
		return
	}
	Logger().With("category", "session").Info(event, "details", details)
}

func Error(category string, err error) {
	if !IsEnabled() {
		return
	}
	Logger().With("category", "error").Error("error", "source", category, "err", err)
}

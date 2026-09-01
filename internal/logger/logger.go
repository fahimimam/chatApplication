// Package logger provides a small structured logger.
//
// The logger is intentionally minimal so that the terminal UI can run the
// process without log lines corrupting the screen. When a TUI is active, log
// output should be redirected to a file by the caller.
package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents a log severity.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "?"
	}
}

// Logger writes structured key/value log lines.
type Logger struct {
	mu    sync.Mutex
	out   io.Writer
	level Level
	now   func() time.Time
}

// New returns a Logger writing to w with the given minimum level.
func New(w io.Writer, level Level) *Logger {
	if w == nil {
		w = os.Stderr
	}
	return &Logger{out: w, level: level, now: time.Now}
}

// Default returns a logger writing to stderr at INFO level.
func Default() *Logger { return New(os.Stderr, LevelInfo) }

// Discard returns a logger that throws everything away. Useful for tests and
// for the TUI process when stdout/stderr are owned by the terminal.
func Discard() *Logger { return New(io.Discard, LevelError) }

func (l *Logger) log(level Level, msg string, kv []any) {
	if level < l.level {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	fmt.Fprintf(l.out, "%s %s", l.now().Format(time.RFC3339), level.String())
	for i := 0; i+1 < len(kv); i += 2 {
		fmt.Fprintf(l.out, " %v=%v", kv[i], kv[i+1])
	}
	fmt.Fprintln(l.out, " "+msg)
}

// Info logs at info level.
func (l *Logger) Info(msg string, kv ...any)  { l.log(LevelInfo, msg, kv) }
func (l *Logger) Warn(msg string, kv ...any)  { l.log(LevelWarn, msg, kv) }
func (l *Logger) Error(msg string, kv ...any) { l.log(LevelError, msg, kv) }
func (l *Logger) Debug(msg string, kv ...any) { l.log(LevelDebug, msg, kv) }

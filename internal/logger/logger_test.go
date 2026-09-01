package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestLoggerRespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelWarn)

	l.Info("ignored")
	l.Warn("kept")
	l.Error("also kept")

	out := buf.String()
	if strings.Contains(out, "ignored") {
		t.Fatalf("info should be filtered at warn level: %q", out)
	}
	if !strings.Contains(out, "kept") || !strings.Contains(out, "also kept") {
		t.Fatalf("expected warn and error to be present, got %q", out)
	}
}

func TestLoggerIncludesKeyValues(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelInfo)

	l.Info("hello", "user", "alice", "count", 3)

	out := buf.String()
	if !strings.Contains(out, "user=alice") || !strings.Contains(out, "count=3") {
		t.Fatalf("expected key/value pairs, got %q", out)
	}
}

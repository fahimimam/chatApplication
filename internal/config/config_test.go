package config

import (
	"os"
	"testing"
	"time"

	"github.com/fahimimam/chatApplication/internal/logger"
)

func TestFromEnvAppliesDefaults(t *testing.T) {
	t.Setenv("CHATD_ADDR", "")
	t.Setenv("CHATD_DB", "")
	t.Setenv("CHATD_LOG_LEVEL", "")
	t.Setenv("CHATTUI_ADDR", "")
	t.Setenv("CHATTUI_TIMEOUT", "")
	c, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if c.ServerAddr != ":3000" {
		t.Fatalf("default addr = %q", c.ServerAddr)
	}
	if c.ClientTimeout != 30*time.Second {
		t.Fatalf("default timeout = %s", c.ClientTimeout)
	}
}

func TestFromEnvInvalidTimeout(t *testing.T) {
	t.Setenv("CHATTUI_TIMEOUT", "not-a-duration")
	if _, err := FromEnv(); err == nil {
		t.Fatal("expected error for invalid timeout")
	}
}

func TestValidateRejectsBadLogLevel(t *testing.T) {
	c := Config{ServerAddr: ":3000", DatabasePath: "x.db", LogLevel: "verbose"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]logger.Level{"debug": logger.LevelDebug, "INFO": logger.LevelInfo, "Warn": logger.LevelWarn, "error": logger.LevelError}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Fatalf("%s -> %d, want %d", in, got, want)
		}
	}
}

func TestGetenvDefault(t *testing.T) {
	os.Unsetenv("CHATD_TEST_UNSET")
	if got := getenv("CHATD_TEST_UNSET", "fallback"); got != "fallback" {
		t.Fatalf("got %q", got)
	}
}

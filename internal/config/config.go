// Package config holds runtime configuration for both the chatd server and the
// chattui client. Values come from environment variables; flags can be added
// later without changing call sites.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fahimimam/chatApplication/internal/logger"
)

// Config is the runtime configuration for a process.
type Config struct {
	// Server-side
	ServerAddr     string // host:port for chatd
	DatabasePath   string // SQLite path
	LogFile        string // optional; if empty, stderr (server) or discard (tui)
	LogLevel       string // "debug" | "info" | "warn" | "error"
	MetricsAddr    string // optional; "" disables the /metrics endpoint

	// Client-side
	ClientServerAddr string // host:port the client connects to
	ClientTimeout    time.Duration
}

// FromEnv reads configuration from environment variables, applying defaults
// suitable for local development.
func FromEnv() (Config, error) {
	cfg := Config{
		ServerAddr:       getenv("CHATD_ADDR", ":3000"),
		DatabasePath:     getenv("CHATD_DB", "chat.db"),
		LogFile:          os.Getenv("CHATD_LOG_FILE"),
		LogLevel:         getenv("CHATD_LOG_LEVEL", "info"),
		MetricsAddr:      os.Getenv("CHATD_METRICS_ADDR"),
		ClientServerAddr: getenv("CHATTUI_ADDR", "127.0.0.1:3000"),
	}

	timeoutStr := getenv("CHATTUI_TIMEOUT", "30s")
	d, err := time.ParseDuration(timeoutStr)
	if err != nil {
		return Config{}, fmt.Errorf("invalid CHATTUI_TIMEOUT %q: %w", timeoutStr, err)
	}
	cfg.ClientTimeout = d

	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	return cfg, nil
}

// Validate checks the configuration for fatal mistakes (bad log level, empty
// addresses). It does not check database reachability.
func (c Config) Validate() error {
	switch strings.ToLower(c.LogLevel) {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid log level %q", c.LogLevel)
	}
	if c.ServerAddr == "" {
		return errors.New("server address is empty")
	}
	if c.DatabasePath == "" {
		return errors.New("database path is empty")
	}
	return nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ParseLevel converts a string log level into a logger.Level.
func ParseLevel(s string) logger.Level {
	switch strings.ToLower(s) {
	case "debug":
		return logger.LevelDebug
	case "info":
		return logger.LevelInfo
	case "warn":
		return logger.LevelWarn
	case "error":
		return logger.LevelError
	}
	return logger.LevelInfo
}

// FormatDuration renders a duration in a human-readable short form.
func FormatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}
	return d.String()
}

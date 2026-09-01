// Command chatd is the chat server. It exposes a TCP line-oriented
// protocol that the chattui (and any future client) can speak.
package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fahimimam/chatApplication/internal/application"
	"github.com/fahimimam/chatApplication/internal/config"
	"github.com/fahimimam/chatApplication/internal/infra/db"
	"github.com/fahimimam/chatApplication/internal/logger"
	"github.com/fahimimam/chatApplication/internal/repository/sqlite"
	"github.com/fahimimam/chatApplication/internal/transport/tcp"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "chatd:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	log, err := buildLogger(cfg)
	if err != nil {
		return err
	}
	log.Info("starting chatd",
		"addr", cfg.ServerAddr,
		"db", cfg.DatabasePath,
		"log_level", cfg.LogLevel,
	)

	conn, err := db.Open(cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer conn.Close()
	if err := db.Migrate(context.Background(), conn); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	repos := sqlite.New(conn)
	auth := application.NewAuthService(repos.Users, repos.Sessions, nil)
	conv := application.NewConversationService(repos.Conversations, repos.Users, nil)
	msg := application.NewMessageService(repos.Messages, repos.Conversations, repos.Users, nil)

	srv := tcp.NewServer(auth, conv, msg, repos.Users, log)

	listener, err := net.Listen("tcp", cfg.ServerAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.ServerAddr, err)
	}
	defer listener.Close()
	log.Info("listening", "addr", cfg.ServerAddr)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := srv.Serve(ctx, listener); err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	log.Info("shutdown complete")
	return nil
}

func buildLogger(cfg config.Config) (*logger.Logger, error) {
	if cfg.LogFile == "" {
		return logger.New(os.Stderr, config.ParseLevel(cfg.LogLevel)), nil
	}
	f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	// We deliberately do not close f; the process owns it for its lifetime.
	_ = time.Now // keep time import if cfg.LogLevel validation uses it later
	return logger.New(f, config.ParseLevel(cfg.LogLevel)), nil
}
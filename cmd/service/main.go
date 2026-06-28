package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/TheEmfield/chat-rooms/internal/config"
	"github.com/TheEmfield/chat-rooms/internal/logger"
	"github.com/TheEmfield/chat-rooms/internal/wsserver"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "c", "config.yaml", "config path")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config parse fail: %s\n", err)
		os.Exit(1)
	}

	logger, err := logger.Setup(&cfg.Logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup logger fail: %s\n", err)
		os.Exit(1)
	}

	if err := run(cfg, logger); err != nil {
		logger.Error(
			"run server",
			"error", err,
		)

		os.Exit(1)
	}
}

func run(cfg *config.Config, logger *slog.Logger) error {
	wsSrv := wsserver.NewWsServer(cfg, logger)
	logger.Info("start server")
	if err := wsSrv.Start(); err != nil {
		return fmt.Errorf("error with wsserver: %v", err)
	}

	crashed := make(chan struct{}, 1)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	select {
	case <-crashed:
		return errors.New("server crashed")

	case <-ctx.Done():
	}

	logger.Info("starting shutdown server")

	ctx, cancel = context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := wsSrv.Stop(); err != nil {
		return fmt.Errorf("shutdown server, %w", err)
	}

	logger.Info("server stopped gracefully")

	return nil
}

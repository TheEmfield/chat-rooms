package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
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

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	crashed := make(chan error, 1)

	go func() {
		logger.Info("start server")
		if err := wsSrv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			crashed <- fmt.Errorf("server crashed: %w", err)
		}
		close(crashed)
	}()

	select {
	case err := <-crashed:
		return err
	case <-ctx.Done():
		logger.Info("received shutdown signal")
	}

	logger.Info("starting shutdown server")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer shutdownCancel()

	if err := wsSrv.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}

	logger.Info("server stopped gracefully")
	return nil
}

package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflowserver"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := taskflowserver.ConfigFromEnv()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	server, err := taskflowserver.New(cfg, logger)
	if err != nil {
		logger.Error("initialize taskflow", "error", err)
		os.Exit(1)
	}

	httpServer := &http.Server{
		Addr:              cfg.Listen,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
	}
	logger.Info("taskflow listening", "address", cfg.Listen, "host_id", cfg.HostID)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("taskflow stopped", "error", err)
		os.Exit(1)
	}
}

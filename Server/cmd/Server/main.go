package main

import (
	srv "Server"
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

const (
	logpath = "/var/log/goexpose"
)

var loglevel = new(slog.LevelVar)
var consoleLogging = flag.Bool("consolelog", false, "Enable console logging")

/*
	STATUS:
		- 2024-04-15: Full rewrite in progress
*/

func main() {
	// Setup logger
	writer := Utils.SetupLoggerWriter(logpath, "server", *consoleLogging)
	logger := slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: loglevel,
	}))

	// GoExpose Server uses a root context to manage shutting down all goroutines
	ctx, cancel := context.WithCancel(context.Background())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// Start the server
	logger.Info("Starting server", "Func", "main")
	server := srv.Server{
		Logger: logger,
	}
	go server.Run(ctx)

	// Wait for signals or context termination
	select {
	case <-signals:
		logger.Info("Received SIGINT/SIGTERM. Closing context and waiting for srv to stop...", "Func", "main")
		cancel()
		break
	case <-ctx.Done():
		break
	}
	logger.Info("Server stopped", "Func", "main")
}

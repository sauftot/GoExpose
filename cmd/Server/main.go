package Server

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var loglevel = new(slog.LevelVar)
var consoleLogging = flag.Bool("consolelog", false, "Enable console logging")

/*
	STATUS:
		- 2024-02-10: Proxying is working.
*/

func setupLoggerWriter() io.Writer {
	// check if goexpose directory exists in /var/logger
	if _, err := os.Stat("/var/logger/goexpose"); os.IsNotExist(err) {
		err = os.Mkdir("/var/logger/goexpose", 0755)
		if err != nil {
			panic("Failed to create /var/logger/goexpose directory: " + err.Error())
		}
	}
	// create logger file
	file, err := os.OpenFile("/var/logger/goexpose/server"+time.Now().Format(time.RFC3339)+".logger", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}
	var writers []io.Writer
	writers = append(writers, file)
	if *consoleLogging {
		writers = append(writers, os.Stdout)
	}

	return io.MultiWriter(writers...)
}

func main() {
	// Setup logger
	writer := setupLoggerWriter()
	logger := slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: loglevel,
	}))

	// GoExpose Server uses a root context to manage shutting down all goroutines, a sub-context will be derived for each
	// open port that gets relayed by the server
	ctx, cancel := context.WithCancel(context.Background())

	// WaitGroup for synchronisation
	var wg sync.WaitGroup

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	server := Server{
		logger: logger,
	}
	wg.Add(1)
	go server.run(ctx, &wg)

	<-signals
	cancel()
	logger.Info("Received SIGINT/SIGTERM. Closing context and waiting for server to stop...")
	wg.Wait()
	logger.Info("Server stopped")
}

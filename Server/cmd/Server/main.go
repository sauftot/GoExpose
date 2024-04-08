package main

import (
	srv "Server"
	"context"
	"flag"
	"io"
	"log/slog"
	"net"
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
	file, err := os.OpenFile("/var/logger/goexpose/srv"+time.Now().Format(time.RFC3339)+".logger", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
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

func setupTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
}

func createConnPair(port int) (*net.TCPConn, *net.TCPConn) {
	ln, err := net.ListenTCP("tcp", &net.TCPAddr{Port: port})
	if err != nil {
		return nil, nil
	}

	conn1, err := net.DialTCP("tcp", nil, &net.TCPAddr{Port: port})
	if err != nil {
		return nil, nil
	}

	conn2, err := ln.AcceptTCP()
	if err != nil {
		return nil, nil
	}

	return conn1, conn2
}

func main() {

	// Setup logger
	writer := setupLoggerWriter()
	logger := slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{
		Level: loglevel,
	}))

	// GoExpose Server uses a root context to manage shutting down all goroutines, a sub-context will be derived for each
	// open port that gets relayed by the srv
	ctx, cancel := context.WithCancel(context.Background())

	// WaitGroup for synchronisation
	var wg sync.WaitGroup

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	server := srv.Server{
		Logger: logger,
	}
	wg.Add(1)
	go server.run(ctx)

	<-signals
	cancel()
	logger.Info("Received SIGINT/SIGTERM. Closing context and waiting for srv to stop...")
	logger.Info("Server stopped")
}

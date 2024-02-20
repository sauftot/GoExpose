package main

import (
	"context"
	mylog "example.com/reverseproxy/pkg/logger"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var wg sync.WaitGroup
var logger *mylog.Logger
var loglevel = mylog.DEBUG

/*
	STATUS:
		- 2024-02-10: Proxying is working.
*/

func main() {
	var err error
	logger, err = mylog.NewLogger("Server")
	logger.SetLogLevel(loglevel)
	if err != nil {
		panic(err)
	}

	// ROOT CONTEXT
	ctx, cancel := context.WithCancel(context.Background())

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	server := NewServer(ctx)
	wg.Add(1)
	go server.run()

	<-signals
	cancel()
	logger.Log("Received SIGINT/SIGTERM. Stopping server...")
	wg.Wait()
	logger.Log("Server stopped")
}

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
	TODO: find why server doesnt shut down properly (missing done() or stop handler)
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
	go func(signals chan os.Signal, cancelFunc context.CancelFunc) {
		for {
			select {
			case <-signals:
				logger.Log("Received signal!")
				cancel()
				return
			}
		}
	}(signals, cancel)

	server := NewServer(ctx)
	wg.Add(1)
	go server.run()

	wg.Wait()
	logger.Log("Server stopped")
}

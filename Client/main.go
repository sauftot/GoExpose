package main

import (
	"context"
	mylog "example.com/reverseproxy/pkg/logger"
	"sync"
)

var wg sync.WaitGroup
var logger *mylog.Logger
var loglevel = mylog.DEBUG

/*
	STATUS:
		- 2024-02-10: Proxying is working.
		//TODO: improve logging, add tests
*/

func main() {
	var err error
	logger, err = mylog.NewLogger("Client")
	logger.SetLogLevel(loglevel)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	input := make(chan []string, 100)

	go internal.InputHandler(cancel, input)
	client := NewClient(ctx)
	wg.Add(1)
	go client.run(input)

	wg.Wait()
	logger.Log("Client stopped")
}

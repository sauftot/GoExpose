package main

import (
	"context"
	"example.com/reverseproxy/pkg/console"
	mylog "example.com/reverseproxy/pkg/logger"
	"sync"
)

var wg sync.WaitGroup
var logger *mylog.Logger
var loglevel = mylog.DEBUG

/*
	TODO: find why client doesnt shut down after it was conected to a server and exit was called (probably: missing done() or stop handler)
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

	go console.InputHandler(cancel, input)
	client := NewClient(ctx)
	wg.Add(1)
	go client.run(input)

	wg.Wait()
	logger.Log("Client stopped")
}

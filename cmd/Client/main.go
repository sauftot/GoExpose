package main

import (
	"context"
	"example.com/reverseproxy/cmd/internal"
	mylog "example.com/reverseproxy/pkg/logger"
	"sync"
)

var wg sync.WaitGroup
var logger *mylog.Logger
var loglevel = mylog.DEBUG

/*
	STATUS:
		- 2024-02-10: Proxying is working.
		//TODO: add functionality to add and remove rules to ufw from the server. This will require root, hence this should depend on a command line argument or a config file.
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

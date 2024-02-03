package main

import (
	"example.com/reverseproxy/pkg/console"
	mylog "example.com/reverseproxy/pkg/logger"
	"sync"
)

var wg sync.WaitGroup
var logger *mylog.Logger
var loglevel = mylog.DEBUG
var stop chan struct{}

/*
	TODO: find why client doesnt shut down after it was conected to a server and exit was called (probably: missing done() or stop handler)
*/

func main() {
	var err error
	logger, err = mylog.NewLogger("Server")
	logger.SetLogLevel(loglevel)
	if err != nil {
		panic(err)
	}

	stop = make(chan struct{})
	input := make(chan []string, 100)

	go console.InputHandler(stop, input)
	client := NewClient()
	wg.Add(1)
	go client.run(input)

	wg.Wait()
	if _, err := <-stop; err {
		close(stop)
	}
	logger.Log("GoExposeServer stopped")
}

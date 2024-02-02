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

func main() {
	var err error
	logger, err = mylog.NewLogger("Server")
	logger.SetLogLevel(loglevel)
	if err != nil {
		panic(err)
	}

	stop = make(chan struct{})
	go console.StopHandler(stop)
	server := NewServer()
	wg.Add(1)
	go server.run()

	wg.Wait()
	if _, err := <-stop; err {
		close(stop)
	}
	logger.Log("GoExposeServer stopped")
}

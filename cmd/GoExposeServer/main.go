package main

import (
	"example.com/reverseproxy/pkg/console"
	mylog "example.com/reverseproxy/pkg/logger"
	"sync"
)

var wg sync.WaitGroup
var logger *mylog.Logger
var debuglevel = mylog.DEBUG
var stop chan struct{}

// DONE
func main() {
	logger, err := mylog.NewLogger("Server")
	logger.SetLogLevel(debuglevel)
	if err != nil {
		panic(err)
	}

	stop := make(chan struct{})
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

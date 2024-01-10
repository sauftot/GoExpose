package main

import (
	"example.com/reverseproxy/pkg/console"
	"sync"
)

// DONE
func main() {
	var wg sync.WaitGroup

	stop := make(chan bool)
	defer close(stop)

	wg.Add(1)
	s := newGeServer(&wg)
	go s.run(stop)
	go console.StopHandler(stop)

	wg.Wait()
}

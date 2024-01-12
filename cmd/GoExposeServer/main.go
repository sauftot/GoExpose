package main

import (
	"example.com/reverseproxy/pkg/console"
	"fmt"
	"sync"
)

// DONE
func main() {
	var wg sync.WaitGroup
	stop := make(chan struct{})
	go console.StopHandler(stop)

	wg.Add(1)
	s := newGeServer(&wg)
	go s.run(stop)

	wg.Wait()
	if _, err := <-stop; err {
		close(stop)
	}
	fmt.Println("MAIN: TERMINATING!")
}

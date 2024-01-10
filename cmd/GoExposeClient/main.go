package main

import (
	"example.com/reverseproxy/pkg/console"
	"sync"
)

// DONE
func main() {
	var wg sync.WaitGroup

	stop := make(chan bool)
	input := make(chan []string)
	defer func(stop chan<- bool, input chan<- []string) {
		close(stop)
		close(input)
	}(stop, input)

	wg.Add(1)
	c := newGeClient(&wg)
	go c.run(stop, input)
	go console.InputHandler(stop, input)

	wg.Wait()
}

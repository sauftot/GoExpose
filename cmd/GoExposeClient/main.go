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
	input := make(chan []string)
	defer close(input)

	go console.InputHandler(stop, input)
	wg.Add(1)
	c := newGeClient(&wg)
	go c.run(stop, input)

	wg.Wait()
	if _, err := <-stop; err {
		close(stop)
	}
	fmt.Println("MAIN: TERMINATING!")
}

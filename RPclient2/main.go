package main

import (
	"example.com/reverseproxy/pkg/console"
	"sync"
)

const (
	CTRLPORT     uint16 = 47921
	UDPPROXYPORT uint16 = 47922
	TCPPROXYBASE uint16 = 47923
	// Set this to the number of tcp ports you want to relay
	NRTCPPORTS uint16 = 10
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
	c := newRPclient(&wg)
	go c.run(stop, input)
	go console.InputHandler(stop, input)

	wg.Wait()
}

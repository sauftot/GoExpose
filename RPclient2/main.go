package main

import (
	"fmt"
	"strings"
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
	go consoleInputHandler(stop, input)

	wg.Wait()
}

// DONE
func consoleInputHandler(stop chan<- bool, input chan<- []string) {
	var cslString string
	for {
		_, err := fmt.Scanln(&cslString)
		if err != nil {
			fmt.Println("CONSOLECONTROLLER: Couldn't read from console!")
			stop <- true
			return
		}
		cslString = strings.ToLower(cslString)
		tokens := strings.Split(cslString, " ")
		switch tokens[0] {
		case "exit":
			fmt.Println("CONSOLECONTROLLER: Received stop command!")
			stop <- true
			return
		default:
			input <- tokens
		}
	}
}

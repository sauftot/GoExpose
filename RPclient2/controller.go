package main

import (
	"fmt"
	"sync"
)

type GeClient struct {
	locPort uint16
	net     *Networker
	wg      *sync.WaitGroup
}

func newRPclient(w *sync.WaitGroup) *GeClient {
	return &GeClient{
		locPort: 0,
		net:     newNetworker(),
		wg:      w,
	}
}

func (c *GeClient) run(stop <-chan bool, input <-chan []string) {
	defer c.wg.Done()
	for {
		select {
		case <-stop:
			return
		case userIO := <-input:
			if c.handleInput(userIO) {
				c.wg.Add(1)
				go c.net.run(stop)
			}
		}
	}
}

/*
helper function for run, handles user console input
*/
func (c *GeClient) handleInput(i []string) bool {
	switch i[0] {
	case "pair":
		if len(i) != 2 {
			fmt.Println("ERROR: handleInput: invalid number of arguments! use 'pair <ip>'")
		} else {
			if !c.net.paired {
				fmt.Println("HANDLEINPUT: Attempting to pair with " + i[1] + ". Please wait!")
				err := c.net.pair(i[1])
				if err != nil {
					fmt.Println("ERROR: handleInput: failed to pair with " + i[1] + "!\n" + err.Error())
				} else {
					fmt.Println("HANDLEINPUT: Successfully paired with " + i[1] + "!")
					return true
				}
			} else {
				fmt.Println("ERROR: handleInput: client is already paired to a server!\n Use 'unpair' to unpair from the current server")
			}
		}
	case "unpair":
		if len(i) != 1 {
			fmt.Println("ERROR: handleInput: invalid number of arguments! use 'unpair'")
		} else {
			if c.net.paired {
				fmt.Println("HANDLEINPUT: Unpairing from server. Please wait!")
				err := c.net.unpair()
				if err != nil {
					fmt.Println("ERROR: handleInput: failed to unpair from server!\n" + err.Error())
				} else {
					fmt.Println("HANDLEINPUT: Successfully unpaired from server!")
				}
			} else {
				fmt.Println("ERROR: handleInput: client is not paired to a server!")
			}
		}
	case "expose":

	case "hide":
	}
	return false
}

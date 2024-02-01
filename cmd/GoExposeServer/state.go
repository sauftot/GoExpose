package main

import "example.com/reverseproxy/pkg/frame"

type State struct {
	Paired       bool
	NetOut       chan *frame.CTRLFrame
	exposedPorts []int
	proxyPorts   []int
}

func NewState() *State {
	return &State{
		Paired:       false,
		NetOut:       make(chan *frame.CTRLFrame, 100),
		exposedPorts: make([]int, 10),
	}
}

func (s *State) ExposeTcp(port int) {
	/*
		Check if the port is already exposed. If not, start a listener on the port and add it to the list of exposed ports.
		Once an external connection comes in, start a listener on a proxy port and send CTRLCONNECT with the proxy port to the client.
		Wait for the client to connect to the proxy port, then hand off the external connection and the connection to the client to relay goroutines.
	*/
}

func (s *State) HideTcp(port int) {
	/*
		Check if the port is being exposed. If so, stop the listener for external connections and send a signal to all relay
		goroutines with this port to stop. Remove the port fro the list of exposed ports.
	*/
}

package main

import (
	"net"
	"sync"
)

type Networker struct {
	wg       *sync.WaitGroup
	paired   bool
	ctrlConn *net.TCPConn
	expTCP   map[uint16]bool
	expUDP   map[uint16]bool
}

func newNetworker(wg *sync.WaitGroup) *Networker {
	return &Networker{
		wg:       wg,
		paired:   false,
		ctrlConn: nil,
		expTCP:   make(map[uint16]bool),
		expUDP:   make(map[uint16]bool),
	}
}

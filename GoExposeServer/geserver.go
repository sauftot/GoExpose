package main

import (
	"example.com/reverseproxy/pkg/frame"
	"net"
	"sync"
)

type GeServer struct {
	paired bool
	wg     *sync.WaitGroup
}

func newGeServer(wg *sync.WaitGroup) *GeServer {
	return &GeServer{
		paired: false,
		wg:     wg,
	}
}

func (s *GeServer) run(stop <-chan bool) {
	for {
		cListen := createControlSocket()
		for !s.paired {
			select {
			case <-stop:
				return

			}
		}

		for s.paired {

		}
	}
}

func createControlSocket() *net.TCPListener {
	sock, err := net.ListenTCP("tcp", &net.TCPAddr{Port: int(frame.CTRLPORT)})
	if err != nil {
		return nil
	}
	return sock
}

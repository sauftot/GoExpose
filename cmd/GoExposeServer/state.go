package main

import (
	"errors"
	"example.com/reverseproxy/pkg/frame"
	"net"
	"strconv"
	"time"
)

type State struct {
	Paired       bool
	PairedIP     net.Addr
	NetOut       chan *frame.CTRLFrame
	exposedPorts map[int]bool
	proxyPorts   []int
}

func NewState() *State {
	return &State{
		Paired:       false,
		exposedPorts: make(map[int]bool),
		proxyPorts:   make([]int, 0),
	}
}

func (s *State) ExposeTcp(port int) {
	/*
		Check if the port is already exposed. If not, start an Exposer for the external port.
	*/
	if port < 1024 || port > 65535 {
		return
	}
	if exposed := s.exposedPorts[port]; exposed {
		return
	}
	if len(s.proxyPorts) >= 10 {
		return
	}

	s.exposedPorts[port] = true
	s.proxyPorts = append(s.proxyPorts, port)
	// Start a listener on the port
	wg.Add(1)
	go s.startExposer(port)
}

func (s *State) HideTcp(port int) {
	/*
		Check if the port is being exposed. If so, stop the listener for external connections and send a signal to all relay
		goroutines with this port to stop. Remove the port fro the list of exposed ports.
	*/
	if port < 1024 || port > 65535 {
		return
	}
	if exposed := s.exposedPorts[port]; !exposed {
		return
	}
	s.exposedPorts[port] = false
}

func (s *State) startExposer(port int) {
	defer wg.Done()
	defer func() {
		for i, p := range s.proxyPorts {
			if p == port {
				s.proxyPorts = append(s.proxyPorts[:i], s.proxyPorts[i+1:]...)
			}
		}
		s.exposedPorts[port] = false
	}()
	// Accept a connection
	// Start a listener on a proxy port
	// Send CTRLCONNECT with the proxy port to the client
	// Wait for the client to connect to the proxy port
	// Hand off the external connection and the connection to the client to relay goroutines
	var proxyPort int
	l, err := net.ListenTCP("tcp", &net.TCPAddr{Port: port})
	if err != nil {
		logger.Error("Error exposer listening:", err)
		return
	}

	for s.exposedPorts[port] && s.Paired {
		select {
		case <-stop:
			return
		default:
			err = l.SetDeadline(time.Now().Add(1 * time.Second))
			if err != nil {
				logger.Error("Error exposer setting deadline:", err)
				return
			}
			extConn, err := l.AcceptTCP()
			if err != nil {
				if netErr := err.(net.Error); netErr.Timeout() {
					// healthy timeout
					continue
				} else {
					logger.Error("Error exposer accepting external connection:", err)
					return
				}
			}
			for i := 0; i < 10; i++ {
				if s.proxyPorts[i] == port {
					proxyPort = TCPPROXYBASE + i
				}
			}
			// Start a listener on the proxy port
			lProxy, err := net.ListenTCP("tcp", &net.TCPAddr{Port: proxyPort})
			if err != nil {
				logger.Error("Error exposer listening on proxy port:", err)
				return
			}
			s.NetOut <- frame.NewCTRLFrame(frame.CTRLCONNECT, []string{strconv.Itoa(port)})

			// Client has 2 seconds to connect to the proxy port
			err = lProxy.SetDeadline(time.Now().Add(2 * time.Second))
			if err != nil {
				logger.Error("Error exposer setting deadline:", err)
				return
			}
			proxConn, err := lProxy.AcceptTCP()
			if err != nil {
				logger.Error("Error exposer accepting proxy connection:", err)
				return
			}
			ip1, _, _ := net.SplitHostPort(proxConn.RemoteAddr().String())
			ip2, _, _ := net.SplitHostPort(s.PairedIP.String())

			if ip1 != ip2 {
				logger.Error("Error: IP mismatch", errors.New("IP mismatch"))
				return
			}
			// hand off the connections to relayTcp
			wg.Add(1)
			go s.relayTcp(extConn, proxConn, port)
			wg.Add(1)
			go s.relayTcp(proxConn, extConn, port)
		}
	}
}

func (s *State) relayTcp(conn1, conn2 *net.TCPConn, port int) {
	defer wg.Done()
	for s.exposedPorts[port] && s.Paired {
		select {
		case <-stop:
			return
		default:
			buf := make([]byte, 1024)
			n, err := conn1.Read(buf)
			if err != nil {
				logger.Error("Error relay reading from external connection:", err)
				return
			}
			_, err = conn2.Write(buf[:n])
			if err != nil {
				logger.Error("Error relay writing to proxy connection:", err)
				return
			}
		}
	}
}

func (s *State) CleanUp() {
	s.Paired = false
	s.PairedIP = nil
	s.proxyPorts = make([]int, 0)
	for k := range s.exposedPorts {
		s.exposedPorts[k] = false
	}
}

package main

import (
	"errors"
	"example.com/reverseproxy/pkg/frame"
	"net"
	"strconv"
	"time"
)

type Proxy struct {
	Paired       bool
	PairedIP     net.Addr
	NetOut       chan *frame.CTRLFrame
	exposedPorts map[int]bool
	proxyPorts   []int
}

func NewState() *Proxy {
	return &Proxy{
		Paired:       false,
		exposedPorts: make(map[int]bool),
		proxyPorts:   make([]int, 0),
	}
}

func (p *Proxy) ExposeTcp(port int) {
	/*
		Check if the port is already exposed. If not, start an Exposer for the external port.
	*/
	if port < 1024 || port > 65535 {
		return
	}
	if exposed := p.exposedPorts[port]; exposed {
		return
	}
	if len(p.proxyPorts) >= 10 {
		return
	}

	p.exposedPorts[port] = true
	p.proxyPorts = append(p.proxyPorts, port)
	// Start a listener on the port
	wg.Add(1)
	go p.startExposer(port)
}

func (p *Proxy) HideTcp(port int) {
	/*
		Check if the port is being exposed. If so, stop the listener for external connections and send a signal to all relay
		goroutines with this port to stop. Remove the port fro the list of exposed ports.
	*/
	if port < 1024 || port > 65535 {
		return
	}
	if exposed := p.exposedPorts[port]; !exposed {
		return
	}
	p.exposedPorts[port] = false
}

func (p *Proxy) startExposer(port int) {
	defer wg.Done()
	defer func() {
		for i, o := range p.proxyPorts {
			if o == port {
				p.proxyPorts = append(p.proxyPorts[:i], p.proxyPorts[i+1:]...)
			}
		}
		p.exposedPorts[port] = false
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

	for p.exposedPorts[port] && p.Paired {
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
				if p.proxyPorts[i] == port {
					proxyPort = TCPPROXYBASE + i
				}
			}
			// Start a listener on the proxy port
			lProxy, err := net.ListenTCP("tcp", &net.TCPAddr{Port: proxyPort})
			if err != nil {
				logger.Error("Error exposer listening on proxy port:", err)
				return
			}
			p.NetOut <- frame.NewCTRLFrame(frame.CTRLCONNECT, []string{strconv.Itoa(port)})

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
			ip2, _, _ := net.SplitHostPort(p.PairedIP.String())

			if ip1 != ip2 {
				logger.Error("Error: IP mismatch", errors.New("IP mismatch"))
				return
			}
			// hand off the connections to relayTcp
			wg.Add(1)
			go p.relayTcp(extConn, proxConn, port)
			wg.Add(1)
			go p.relayTcp(proxConn, extConn, port)
		}
	}
}

func (p *Proxy) relayTcp(conn1, conn2 *net.TCPConn, port int) {
	defer wg.Done()
	for p.exposedPorts[port] && p.Paired {
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

func (p *Proxy) CleanUp() {
	p.Paired = false
	p.PairedIP = nil
	p.proxyPorts = make([]int, 0)
	for k := range p.exposedPorts {
		p.exposedPorts[k] = false
	}
}

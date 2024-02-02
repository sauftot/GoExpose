package main

import (
	"crypto/tls"
	"example.com/reverseproxy/pkg/frame"
	"fmt"
	"net"
	"strconv"
)

type Proxy struct {
	Paired         bool
	ip             net.IP
	config         *tls.Config
	exposedPorts   map[int]bool
	exposedPortsNr int
}

func NewProxy() *Proxy {
	return &Proxy{
		Paired:         false,
		ip:             nil,
		config:         nil,
		exposedPorts:   make(map[int]bool),
		exposedPortsNr: 0,
	}
}

func (p *Proxy) setConfig(config *tls.Config) {
	p.config = config
}

func (p *Proxy) connectToServer(domainOrIp string) {
	ip := net.ParseIP(domainOrIp)
	if ip == nil {
		ip2, err := net.ResolveIPAddr("ip4", domainOrIp)
		if err != nil {
			logger.Error("Error resolving domain name: ", err)
			return
		}
		p.ip = ip2.IP
	} else {
		p.ip = ip
	}
	fmt.Println("[INFO] Connecting to server: ", p.ip.String())
	conn, err := tls.Dial("tcp", p.ip.String()+":"+strconv.Itoa(CTRLPORT), p.config)
	if err != nil {
		logger.Error("Error connecting to server: ", err)
		return
	}
	p.Paired = true
	// spin off a go routine to handle the connection
	wg.Add(1)
	go p.handleServerConnection(conn)
}

func (p *Proxy) handleServerConnection(conn *tls.Conn) {
	defer wg.Done()
	defer func(conn *tls.Conn) {
		err := conn.Close()
		if err != nil {
			return
		}
	}(conn)
	for p.Paired {
		fr, err := frame.ReadFrame(conn)
		if err != nil {
			logger.Error("Error reading frame from server: ", err)
			return
		}
		switch fr.Typ {
		case frame.CTRLUNPAIR:

		case frame.CTRLCONNECT:

		}
	}
}

func (p *Proxy) expose(portStr string) {
	if p.exposedPortsNr >= 10 {
		fmt.Println("[ERROR] Maximum number of exposed ports reached!")
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		fmt.Println("[ERROR] Invalid port number!")
		return
	}
}

func (p *Proxy) hide(port string) {

}
